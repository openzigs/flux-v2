// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"miniflux.app/v2/internal/model"
)

func TestTruncateStringForTSVectorField(t *testing.T) {
	const megabyte = 1024 * 1024

	// Test case 1: Short Chinese text should not be truncated
	shortText := "这是一个简短的中文测试文本"
	result := truncateStringForTSVectorField(shortText, megabyte)
	if result != shortText {
		t.Errorf("Short text should not be truncated, got %s", result)
	}

	// Test case 2: Long Chinese text should be truncated to stay under 1MB
	// Generate a long Chinese string that would exceed 1MB
	chineseChar := "汉"
	longText := strings.Repeat(chineseChar, megabyte/len(chineseChar)+1000) // Ensure it exceeds 1MB

	result = truncateStringForTSVectorField(longText, megabyte)

	// Verify the result is under 1MB
	if len(result) >= megabyte {
		t.Errorf("Truncated text should be under 1MB, got %d bytes", len(result))
	}

	// Verify the result is still valid UTF-8 and doesn't cut in the middle of a character
	if !strings.HasPrefix(longText, result) {
		t.Error("Truncated text should be a prefix of original text")
	}

	// Test case 3: Text exactly at limit should not be truncated
	limitText := strings.Repeat("a", megabyte-1)
	result = truncateStringForTSVectorField(limitText, megabyte)
	if result != limitText {
		t.Error("Text under limit should not be truncated")
	}

	// Test case 4: Mixed Chinese and ASCII text
	mixedText := strings.Repeat("测试Test汉字", megabyte/20) // Create large mixed text
	result = truncateStringForTSVectorField(mixedText, megabyte)

	if len(result) >= megabyte {
		t.Errorf("Mixed text should be truncated under 1MB, got %d bytes", len(result))
	}

	// Verify no broken UTF-8 sequences
	if !strings.HasPrefix(mixedText, result) {
		t.Error("Truncated mixed text should be a valid prefix")
	}

	// Test case 5: Large text ending with ASCII characters
	asciiSuffix := strings.Repeat("a", megabyte-100) + strings.Repeat("测试", 50) + "abcdef"
	result = truncateStringForTSVectorField(asciiSuffix, megabyte)

	if len(result) >= megabyte {
		t.Errorf("ASCII suffix text should be truncated under 1MB, got %d bytes", len(result))
	}

	// Should end with ASCII character
	if !strings.HasPrefix(asciiSuffix, result) {
		t.Error("Truncated ASCII suffix text should be a valid prefix")
	}

	// Test case 6: Large ASCII text to cover ASCII branch in UTF-8 detection
	largeAscii := strings.Repeat("abcdefghijklmnopqrstuvwxyz", megabyte/26+1000)
	result = truncateStringForTSVectorField(largeAscii, megabyte)

	if len(result) >= megabyte {
		t.Errorf("Large ASCII text should be truncated under 1MB, got %d bytes", len(result))
	}

	// Should be a prefix
	if !strings.HasPrefix(largeAscii, result) {
		t.Error("Truncated ASCII text should be a valid prefix")
	}

	// Test case 7: Edge case - string that would trigger the fallback
	// Create a pathological case: all continuation bytes without start bytes
	// This should trigger the fallback because there are no valid UTF-8 boundaries
	invalidBytes := make([]byte, megabyte)
	for i := range invalidBytes {
		invalidBytes[i] = 0x80 // Continuation byte without start byte
	}
	result = truncateStringForTSVectorField(string(invalidBytes), megabyte)

	// Should return empty string as fallback
	if result != "" {
		t.Errorf("Invalid UTF-8 continuation bytes should return empty string, got %d bytes", len(result))
	}
}

// fakeExecDriver is a database/sql driver whose Exec returns a fixed result,
// so the storage layer's handling of RowsAffected can be tested without PostgreSQL.
type fakeExecDriver struct {
	rowsAffected int64
	rowsErr      error
	lastQuery    string
	lastArgs     []driver.NamedValue
}

func (d *fakeExecDriver) Open(string) (driver.Conn, error) { return &fakeExecConn{d}, nil }

type fakeExecConn struct{ d *fakeExecDriver }

func (c *fakeExecConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (c *fakeExecConn) Close() error                        { return nil }
func (c *fakeExecConn) Begin() (driver.Tx, error)           { return nil, errors.New("not supported") }
func (c *fakeExecConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.d.lastQuery = query
	c.d.lastArgs = args
	return fakeResult{c.d.rowsAffected, c.d.rowsErr}, nil
}

type fakeResult struct {
	n   int64
	err error
}

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return r.n, r.err }

type fakeConnector struct{ d *fakeExecDriver }

func (c fakeConnector) Connect(context.Context) (driver.Conn, error) { return &fakeExecConn{c.d}, nil }
func (c fakeConnector) Driver() driver.Driver                        { return c.d }

func newFakeStorage(d *fakeExecDriver) *Storage {
	return NewStorage(sql.OpenDB(fakeConnector{d}))
}

func TestMarkAllAsReadBeforeDateReturnsAffectedCount(t *testing.T) {
	d := &fakeExecDriver{rowsAffected: 42}
	s := newFakeStorage(d)
	before := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	count, err := s.MarkAllAsReadBeforeDate(7, before)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 42 {
		t.Fatalf("expected 42 affected entries, got %d", count)
	}
	if !strings.Contains(d.lastQuery, "published_at < $4") {
		t.Fatalf("expected a strict published_at < cutoff, got query %q", d.lastQuery)
	}
	if !strings.Contains(d.lastQuery, "status=$3") {
		t.Fatalf("expected an unread-only filter, got query %q", d.lastQuery)
	}
	if len(d.lastArgs) != 4 || d.lastArgs[1].Value != int64(7) || d.lastArgs[2].Value != model.EntryStatusUnread {
		t.Fatalf("unexpected query arguments: %v", d.lastArgs)
	}
	if got, ok := d.lastArgs[3].Value.(time.Time); !ok || !got.Equal(before) {
		t.Fatalf("expected the cutoff to be passed unchanged, got %v", d.lastArgs[3].Value)
	}
}

func TestMarkAllAsReadBeforeDatePropagatesRowsAffectedError(t *testing.T) {
	s := newFakeStorage(&fakeExecDriver{rowsErr: errors.New("boom")})

	if _, err := s.MarkAllAsReadBeforeDate(1, time.Now()); err == nil {
		t.Fatal("expected the RowsAffected error to be returned")
	}
}

func TestMarkAllAsReadBeforeDateStrictBoundary(t *testing.T) {
	s := newTestStorage(t)
	f := newTestFixture(t, s)

	cutoff := time.Now().Add(-30 * 24 * time.Hour).Truncate(time.Second)
	older := insertEntry(t, s, f, cutoff.Add(-time.Second), model.EntryStatusUnread)
	exact := insertEntry(t, s, f, cutoff, model.EntryStatusUnread)
	newer := insertEntry(t, s, f, cutoff.Add(time.Second), model.EntryStatusUnread)

	count, err := s.MarkAllAsReadBeforeDate(f.userID, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 affected entry, got %d", count)
	}
	for id, want := range map[int64]string{older: model.EntryStatusRead, exact: model.EntryStatusUnread, newer: model.EntryStatusUnread} {
		if got, _ := entryStatusAndChangedAt(t, s, id); got != want {
			t.Errorf("entry %d: expected status %q, got %q", id, want, got)
		}
	}
}

func TestMarkAllAsReadBeforeDateRepeatChangesNothing(t *testing.T) {
	s := newTestStorage(t)
	f := newTestFixture(t, s)

	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	alreadyRead := insertEntry(t, s, f, cutoff.Add(-time.Hour), model.EntryStatusRead)
	insertEntry(t, s, f, cutoff.Add(-time.Hour), model.EntryStatusUnread)
	_, readChangedAt := entryStatusAndChangedAt(t, s, alreadyRead)

	if count, err := s.MarkAllAsReadBeforeDate(f.userID, cutoff); err != nil || count != 1 {
		t.Fatalf("first run: expected 1 affected entry, got %d (%v)", count, err)
	}
	if count, err := s.MarkAllAsReadBeforeDate(f.userID, cutoff); err != nil || count != 0 {
		t.Fatalf("second run: expected 0 affected entries, got %d (%v)", count, err)
	}
	if _, changedAt := entryStatusAndChangedAt(t, s, alreadyRead); !changedAt.Equal(readChangedAt) {
		t.Fatalf("already-read entry changed_at moved from %v to %v", readChangedAt, changedAt)
	}
}

func TestMarkAllAsReadBeforeDateAbsoluteCutoffResultSet(t *testing.T) {
	s := newTestStorage(t)
	f := newTestFixture(t, s)

	cutoff := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	var stale []int64
	for _, d := range []time.Duration{time.Hour, 24 * time.Hour, 400 * 24 * time.Hour} {
		stale = append(stale, insertEntry(t, s, f, cutoff.Add(-d), model.EntryStatusUnread))
	}
	fresh := insertEntry(t, s, f, cutoff.Add(time.Hour), model.EntryStatusUnread)

	count, err := s.MarkAllAsReadBeforeDate(f.userID, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if count != int64(len(stale)) {
		t.Fatalf("expected %d affected entries, got %d", len(stale), count)
	}
	for _, id := range stale {
		if got, _ := entryStatusAndChangedAt(t, s, id); got != model.EntryStatusRead {
			t.Errorf("entry %d: expected read, got %q", id, got)
		}
	}
	if got, _ := entryStatusAndChangedAt(t, s, fresh); got != model.EntryStatusUnread {
		t.Errorf("fresh entry: expected unread, got %q", got)
	}
}
