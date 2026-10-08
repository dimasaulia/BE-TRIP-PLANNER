package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/calendar/repositories"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/crypto"
	"github.com/open-suite/boilerplate-golang/internal/platform/google"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
)

const (
	pollInterval = 2 * time.Second
	batchSize    = 10
	maxAttempts  = 8
	baseBackoff  = 30 * time.Second
	maxBackoff   = time.Hour
)

// Worker drains sync_jobs and pushes the changes to Google Calendar.
type Worker struct {
	repo     repositories.CalendarRepository
	provider google.CalendarProvider
	keys     *crypto.Keys
	baseURL  string
	log      *logger.LayerLogger
}

func NewWorker(repo repositories.CalendarRepository, googleClient *google.Client, keys *crypto.Keys, cfg config.Config, appLogger *logger.Logger) *Worker {
	return NewWorkerWithProvider(repo, googleClient, keys, cfg.App.BaseURL, appLogger)
}

// NewWorkerWithProvider lets tests substitute a fake Google.
func NewWorkerWithProvider(repo repositories.CalendarRepository, provider google.CalendarProvider, keys *crypto.Keys, baseURL string, appLogger *logger.Logger) *Worker {
	return &Worker{
		repo:     repo,
		provider: provider,
		keys:     keys,
		baseURL:  baseURL,
		log:      appLogger.Layer("worker.calendar"),
	}
}

func (w *Worker) Run(ctx context.Context) {
	w.log.Info(ctx, "worker.start")
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info(ctx, "worker.stop")
			return
		case <-ticker.C:
			// Keep going while there is a backlog instead of waiting a tick per batch.
			for {
				processed, err := w.RunBatch(ctx)
				if err != nil {
					w.log.Error(ctx, "batch.failed", err)
					break
				}
				if processed < batchSize || ctx.Err() != nil {
					break
				}
			}
		}
	}
}

// RunBatch claims and processes up to batchSize due jobs.
func (w *Worker) RunBatch(ctx context.Context) (int, error) {
	jobs, err := w.repo.ClaimJobs(ctx, batchSize)
	if err != nil {
		return 0, err
	}

	for _, job := range jobs {
		w.process(ctx, job)
	}
	return len(jobs), nil
}

func (w *Worker) process(ctx context.Context, job entities.SyncJob) {
	sync, err := w.repo.FindSyncByID(ctx, job.SyncID)
	if err != nil {
		w.retry(ctx, nil, job, err)
		return
	}
	if sync == nil {
		_ = w.repo.FinishJob(ctx, job.ID, job.Seq)
		return
	}

	connection, err := w.repo.FindConnection(ctx, sync.UserID)
	if err != nil {
		w.retry(ctx, sync, job, err)
		return
	}
	if connection == nil || connection.Status != entities.CalendarStatusActive {
		w.fail(ctx, sync, job, "Google Calendar access was revoked; reconnect to resume syncing")
		return
	}

	refreshToken, err := w.keys.Decrypt(connection.RefreshTokenEnc)
	if err != nil {
		w.fail(ctx, sync, job, "stored Google credentials cannot be decrypted; reconnect Google Calendar")
		return
	}

	api := w.provider.Calendar(ctx, string(refreshToken))

	switch job.Op {
	case entities.SyncOpUpsert:
		err = w.upsert(ctx, api, sync, job.BlockID)
	case entities.SyncOpDelete:
		err = w.remove(ctx, api, sync, job.BlockID)
	case entities.SyncOpFullResync:
		err = w.fullResync(ctx, sync)
	default:
		err = fmt.Errorf("unknown sync op %q", job.Op)
	}

	switch {
	case err == nil:
		now := time.Now()
		_ = w.repo.SetSyncResult(ctx, sync.ID, &now, nil)
		_ = w.repo.FinishJob(ctx, job.ID, job.Seq)
	case errors.Is(err, google.ErrInvalidGrant):
		w.log.Warn(ctx, "grant.revoked", "user_id", sync.UserID)
		_ = w.repo.MarkRevoked(ctx, sync.UserID, "Google Calendar access was revoked; reconnect to resume syncing")
	default:
		w.retry(ctx, sync, job, err)
	}
}

func (w *Worker) upsert(ctx context.Context, api google.CalendarAPI, sync *entities.CalendarSync, blockID uuid.UUID) error {
	data, err := w.repo.FindBlockData(ctx, blockID)
	if err != nil {
		return err
	}
	if data == nil {
		// The block is gone, so what Google needs is a delete.
		return w.remove(ctx, api, sync, blockID)
	}

	link, err := w.repo.FindLink(ctx, sync.ID, blockID)
	if err != nil {
		return err
	}

	event := BuildEvent(sync.ID, *data, w.baseURL)
	hash := ContentHash(event)
	if link != nil && link.ContentHash == hash {
		return nil
	}

	if link == nil {
		err = api.InsertEvent(ctx, sync.GoogleCalendarID, event)
		// 409: the deterministic id already exists, so this is an update.
		if apiErr := asAPIError(err); apiErr != nil && apiErr.Status == 409 {
			err = api.UpdateEvent(ctx, sync.GoogleCalendarID, event)
		}
	} else {
		err = api.UpdateEvent(ctx, sync.GoogleCalendarID, event)
		// 404/410: someone deleted the event in Google, so create it again.
		if apiErr := asAPIError(err); apiErr != nil && apiErr.Gone() {
			err = api.InsertEvent(ctx, sync.GoogleCalendarID, event)
		}
	}
	if err != nil {
		return err
	}

	return w.repo.UpsertLink(ctx, entities.CalendarEventLink{
		SyncID:        sync.ID,
		BlockID:       blockID,
		GoogleEventID: event.ID,
		ContentHash:   hash,
	})
}

func (w *Worker) remove(ctx context.Context, api google.CalendarAPI, sync *entities.CalendarSync, blockID uuid.UUID) error {
	link, err := w.repo.FindLink(ctx, sync.ID, blockID)
	if err != nil {
		return err
	}
	if link == nil {
		return nil
	}

	err = api.DeleteEvent(ctx, sync.GoogleCalendarID, link.GoogleEventID)
	if apiErr := asAPIError(err); apiErr != nil && apiErr.Gone() {
		err = nil
	}
	if err != nil {
		return err
	}

	return w.repo.DeleteLink(ctx, sync.ID, blockID)
}

// fullResync fans out into one upsert per current block and one delete per
// orphaned link, so each piece retries on its own.
func (w *Worker) fullResync(ctx context.Context, sync *entities.CalendarSync) error {
	blocks, err := w.repo.ListTripBlockIDs(ctx, sync.TripID)
	if err != nil {
		return err
	}
	present := make(map[uuid.UUID]struct{}, len(blocks))
	for _, id := range blocks {
		present[id] = struct{}{}
		if err := w.repo.EnqueueJob(ctx, sync.ID, id, entities.SyncOpUpsert, 0); err != nil {
			return err
		}
	}

	linked, err := w.repo.ListLinkedBlockIDs(ctx, sync.ID)
	if err != nil {
		return err
	}
	for _, id := range linked {
		if _, ok := present[id]; ok {
			continue
		}
		if err := w.repo.EnqueueJob(ctx, sync.ID, id, entities.SyncOpDelete, 0); err != nil {
			return err
		}
	}
	return nil
}

// retry applies exponential backoff for transient failures and gives up for
// permanent ones, recording the reason on the sync for the UI.
func (w *Worker) retry(ctx context.Context, sync *entities.CalendarSync, job entities.SyncJob, cause error) {
	message := cause.Error()

	if apiErr := asAPIError(cause); apiErr != nil && !apiErr.Retryable() {
		w.fail(ctx, sync, job, message)
		return
	}

	attempts := job.Attempts + 1
	if attempts >= maxAttempts {
		w.fail(ctx, sync, job, "gave up after repeated failures: "+message)
		return
	}

	delay := baseBackoff << (attempts - 1)
	if delay > maxBackoff {
		delay = maxBackoff
	}

	w.log.Warn(ctx, "job.retry", "job_id", job.ID, "attempts", attempts, "delay", delay.String(), "error", message)
	if err := w.repo.RetryJob(ctx, job.ID, job.Seq, attempts, time.Now().Add(delay), message); err != nil {
		w.log.Error(ctx, "job.retry.failed", err)
	}
	if sync != nil {
		_ = w.repo.SetSyncResult(ctx, sync.ID, nil, &message)
	}
}

func (w *Worker) fail(ctx context.Context, sync *entities.CalendarSync, job entities.SyncJob, message string) {
	w.log.Warn(ctx, "job.failed", "job_id", job.ID, "error", message)
	_ = w.repo.FinishJob(ctx, job.ID, job.Seq)
	if sync != nil {
		_ = w.repo.SetSyncResult(ctx, sync.ID, nil, &message)
	}
}

func asAPIError(err error) *google.APIError {
	var apiErr *google.APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return nil
}
