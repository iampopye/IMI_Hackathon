package upsert

import (
	"context"
	"sync"
	"time"
)

// BulkUpsert is the public entry point for inserting or updating records at any volume.
//
// Flow:
//  1. Compute deterministic IDs and checksums for all records (CPU-only, no DB).
//  2. Split into batches of cfg.BatchSize.
//  3. Fan out to cfg.WorkerCount parallel goroutines.
//  4. Each worker calls processBatch — one atomic transaction per batch.
//  5. Aggregate results and return.
//
// At 20M records with BatchSize=1000 and WorkerCount=10:
//
//	20,000 batches ÷ 10 workers → ~2,000 transactions per worker → ~4–6 minutes total.
func (e *UpsertEngine) BulkUpsert(ctx context.Context, datasetID string, records []Record) UpsertResult {
	start := time.Now()

	// Compute IDs and checksums upfront (no DB round-trip needed).
	prepareRecords(datasetID, records)

	batches := splitBatches(records, e.cfg.BatchSize)

	// Buffered work channel — all batches pre-loaded so workers never block on send.
	work := make(chan []Record, len(batches))
	for _, b := range batches {
		work <- b
	}
	close(work)

	var (
		mu     sync.Mutex
		totals UpsertResult
	)
	totals.DatasetID = datasetID
	totals.Total = len(records)

	workers := e.cfg.WorkerCount
	if workers <= 0 {
		workers = 1
	}

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range work {
				r := e.processBatch(ctx, datasetID, batch)
				mu.Lock()
				totals.Inserted += r.inserted
				totals.Updated += r.updated
				totals.Skipped += r.skipped
				totals.Failed += r.failed
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	totals.DurationMs = time.Since(start).Milliseconds()
	return totals
}
