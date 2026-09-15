package stats

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	if s.NewChats != 0 {
		t.Errorf("expected 0, got %d", s.NewChats)
	}
}

func TestIncrementNewChats(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.IncrementNewChats()
	s.IncrementNewChats()
	if s.NewChats != 2 {
		t.Errorf("expected 2, got %d", s.NewChats)
	}
}

func TestIncrementProcessed(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.IncrementProcessed()
	s.IncrementProcessed()
	s.IncrementProcessed()
	if s.TotalProcessed != 3 {
		t.Errorf("expected 3, got %d", s.TotalProcessed)
	}
}

func TestIncrementSuccess(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.IncrementSuccess(1, "youtube.com", 10*1024*1024, 5000)
	s.IncrementSuccess(1, "youtube.com", 20*1024*1024, 3000)

	if s.TotalSuccess != 2 {
		t.Errorf("expected 2, got %d", s.TotalSuccess)
	}
	if s.TopDomains["youtube.com"] != 2 {
		t.Errorf("expected 2, got %d", s.TopDomains["youtube.com"])
	}
	if len(s.FileSizes) != 2 {
		t.Errorf("expected 2 file sizes, got %d", len(s.FileSizes))
	}
	if len(s.ProcessingTimesMs) != 2 {
		t.Errorf("expected 2 times, got %d", len(s.ProcessingTimesMs))
	}
	cs := s.PerChat[1]
	if cs == nil {
		t.Fatal("expected chat 1 stats")
	}
	if cs.Success != 2 {
		t.Errorf("expected 2 success for chat 1, got %d", cs.Success)
	}
}

func TestIncrementFailed(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.IncrementFailed(1, "invalid_url")
	s.IncrementFailed(1, "invalid_url")
	s.IncrementFailed(2, "download_error")

	if s.TotalFailed != 3 {
		t.Errorf("expected 3, got %d", s.TotalFailed)
	}
	if s.ErrorStats["invalid_url"] != 2 {
		t.Errorf("expected 2, got %d", s.ErrorStats["invalid_url"])
	}
	if s.ErrorStats["download_error"] != 1 {
		t.Errorf("expected 1, got %d", s.ErrorStats["download_error"])
	}
}

func TestPerChatStats(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.IncrementSuccess(1, "youtube.com", 100, 1000)
	s.IncrementFailed(1, "err1")
	s.IncrementSuccess(2, "vimeo.com", 200, 2000)

	cs1 := s.PerChat[1]
	if cs1.Success != 1 || cs1.Failed != 1 {
		t.Errorf("chat 1: expected 1/1, got %d/%d", cs1.Success, cs1.Failed)
	}
	cs2 := s.PerChat[2]
	if cs2.Success != 1 || cs2.Failed != 0 {
		t.Errorf("chat 2: expected 1/0, got %d/%d", cs2.Success, cs2.Failed)
	}
}

func TestReport(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.IncrementNewChats()
	s.IncrementProcessed()
	s.IncrementProcessed()
	s.IncrementSuccess(1, "youtube.com", 15*1024*1024, 4000)
	s.IncrementFailed(1, "invalid_url")

	report := s.Report()
	checks := []string{
		"Статистика бота",
		"Новых чатов: 1",
		"Обработано ссылок: 2",
		"Успешно: 1 (50.0%)",
		"Неуспешно: 1 (50.0%)",
		"youtube.com: 1",
		"invalid_url: 1",
		"Средний размер файла: 15.0 MB",
		"Среднее время: 4.0 сек",
	}
	for _, check := range checks {
		if !contains(report, check) {
			t.Errorf("report missing: %s\nfull report:\n%s", check, report)
		}
	}
}

func TestReportEmpty(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	report := s.Report()
	if report == "" {
		t.Error("expected non-empty report")
	}
}

func TestMedianOdd(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.ProcessingTimesMs = []int64{1, 3, 2}
	if got := s.median(); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}

func TestMedianEven(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.ProcessingTimesMs = []int64{4, 1, 3, 2}
	if got := s.median(); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}

func TestMedianEmpty(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	if got := s.median(); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.json")

	s1 := New(path)
	s1.IncrementNewChats()
	s1.IncrementProcessed()
	s1.TrackChat(1, "vasya")
	s1.IncrementSuccess(1, "example.com", 100, 500)
	s1.IncrementFailed(1, "err_type")

	if err := s1.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	s2 := New(path)

	if s2.NewChats != 1 {
		t.Errorf("NewChats: expected 1, got %d", s2.NewChats)
	}
	if s2.TotalProcessed != 1 {
		t.Errorf("TotalProcessed: expected 1, got %d", s2.TotalProcessed)
	}
	if s2.TotalSuccess != 1 {
		t.Errorf("TotalSuccess: expected 1, got %d", s2.TotalSuccess)
	}
	if s2.TotalFailed != 1 {
		t.Errorf("TotalFailed: expected 1, got %d", s2.TotalFailed)
	}
	if s2.TopDomains["example.com"] != 1 {
		t.Errorf("TopDomains: expected 1, got %d", s2.TopDomains["example.com"])
	}
	if cs := s2.PerChat[1]; cs == nil || cs.Username != "vasya" {
		t.Errorf("PerChat username: expected vasya, got %v", cs)
	}
	if got := s2.UserDomains["@vasya"]; len(got) != 1 || got[0] != "example.com" {
		t.Errorf("UserDomains: expected [example.com], got %v", got)
	}
	if s2.ErrorStats["err_type"] != 1 {
		t.Errorf("ErrorStats: expected 1, got %d", s2.ErrorStats["err_type"])
	}
	if len(s2.FileSizes) != 1 || s2.FileSizes[0] != 100 {
		t.Errorf("FileSizes: expected [100], got %v", s2.FileSizes)
	}
}

func TestLoadNonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	s := New(path)
	if s == nil {
		t.Fatal("expected non-nil stats")
	}
}

func TestConcurrency(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.IncrementNewChats()
			s.IncrementProcessed()
			s.IncrementSuccess(1, "youtube.com", 100, 1000)
			s.IncrementFailed(1, "err")
		}()
	}

	wg.Wait()

	if s.NewChats != 50 {
		t.Errorf("NewChats: expected 50, got %d", s.NewChats)
	}
	if s.TotalProcessed != 50 {
		t.Errorf("TotalProcessed: expected 50, got %d", s.TotalProcessed)
	}
	if s.TotalSuccess != 50 {
		t.Errorf("TotalSuccess: expected 50, got %d", s.TotalSuccess)
	}
	if s.TotalFailed != 50 {
		t.Errorf("TotalFailed: expected 50, got %d", s.TotalFailed)
	}
}

func TestConcurrentSaveAndIncrement(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	go s.StartPeriodicSave(ctx, 10*time.Millisecond)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.IncrementNewChats()
		}()
	}

	wg.Wait()
	cancel()

	time.Sleep(50 * time.Millisecond)

	if s.NewChats != 100 {
		t.Errorf("NewChats: expected 100, got %d", s.NewChats)
	}
}

func TestSave_CreateError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0755)

	s := New(filepath.Join(dir, "stats.json"))
	if err := s.Save(); err == nil {
		t.Error("expected error saving to readonly dir")
	}
}

func TestLoad_CorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.json")
	if err := os.WriteFile(path, []byte("{invalid json}"), 0644); err != nil {
		t.Fatal(err)
	}
	s := New(path)
	if s == nil {
		t.Fatal("expected non-nil stats")
	}
}

func TestLoad_NilMaps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nil_maps.json")
	jsonData := `{"new_chats": 5, "per_chat": null, "top_domains": null, "user_domains": null, "error_stats": null, "file_sizes": null, "processing_times_ms": null}`
	if err := os.WriteFile(path, []byte(jsonData), 0644); err != nil {
		t.Fatal(err)
	}
	s := New(path)
	if s.NewChats != 5 {
		t.Errorf("NewChats: expected 5, got %d", s.NewChats)
	}
	if s.PerChat == nil {
		t.Error("PerChat should be initialized")
	}
	if s.TopDomains == nil {
		t.Error("TopDomains should be initialized")
	}
	if s.UserDomains == nil {
		t.Error("UserDomains should be initialized")
	}
	if s.ErrorStats == nil {
		t.Error("ErrorStats should be initialized")
	}
	if s.FileSizes == nil {
		t.Error("FileSizes should be initialized")
	}
	if s.ProcessingTimesMs == nil {
		t.Error("ProcessingTimesMs should be initialized")
	}
}

func TestLoad_NilPerChatValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nil_values.json")
	jsonData := `{"per_chat": {"1": {"success": 2, "failed": 1}, "2": null, "3": null}}`
	if err := os.WriteFile(path, []byte(jsonData), 0644); err != nil {
		t.Fatal(err)
	}
	s := New(path)
	if len(s.PerChat) != 1 {
		t.Fatalf("expected 1 chat after cleaning nil values, got %d", len(s.PerChat))
	}
	if cs := s.PerChat[1]; cs == nil || cs.Success != 2 {
		t.Errorf("chat 1 should have 2 success, got %v", cs)
	}
}

func TestReport_NilPerChatValues(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.PerChat[1] = &ChatStats{Success: 1, Failed: 0}
	s.PerChat[2] = nil
	report := s.Report()
	if !contains(report, "chat 1") {
		t.Errorf("expected chat 1 in report: %s", report)
	}
}

func TestPerChat_ExistingNilValue(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.PerChat[1] = nil
	s.IncrementSuccess(1, "youtube.com", 100, 1000)
	cs := s.PerChat[1]
	if cs == nil || cs.Success != 1 {
		t.Errorf("expected recreated chat stats, got %v", cs)
	}
}

func TestReport_NoProcessed(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.IncrementNewChats()
	report := s.Report()
	if !contains(report, "Новых чатов: 1") {
		t.Errorf("expected new chats in report, got: %s", report)
	}
	if contains(report, "Успешно:") {
		t.Errorf("should not contain success rate when no links processed: %s", report)
	}
}

func TestIncrementSuccess_EmptyDomain(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.IncrementSuccess(1, "", 100, 1000)
	if s.TotalSuccess != 1 {
		t.Errorf("expected 1, got %d", s.TotalSuccess)
	}
	if len(s.TopDomains) != 0 {
		t.Errorf("expected empty domains, got %v", s.TopDomains)
	}
}

func TestIncrementFailed_EmptyErrType(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.IncrementFailed(1, "")
	if s.TotalFailed != 1 {
		t.Errorf("expected 1, got %d", s.TotalFailed)
	}
	if len(s.ErrorStats) != 0 {
		t.Errorf("expected empty errors, got %v", s.ErrorStats)
	}
}

func TestReport_TopDomainsLimit(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	for i := 0; i < 15; i++ {
		domain := string(rune('a' + i)) + ".com"
		s.IncrementSuccess(1, domain, 100, 1000)
	}
	report := s.Report()
	count := strings.Count(report, ".com:")
	if count > 10 {
		t.Errorf("expected at most 10 domains in report, got %d", count)
	}
}

func TestReport_TopDomainsLessThanLimit(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.IncrementSuccess(1, "example.com", 100, 1000)
	s.IncrementSuccess(1, "test.com", 200, 2000)
	report := s.Report()
	if !contains(report, "example.com: 1") {
		t.Errorf("expected example.com in report: %s", report)
	}
	if !contains(report, "test.com: 1") {
		t.Errorf("expected test.com in report: %s", report)
	}
}

func TestReport_MultipleChats(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	for id := int64(1); id <= 3; id++ {
		s.IncrementSuccess(id, "example.com", 100, 1000)
		s.IncrementFailed(id, "err")
	}
	report := s.Report()
	for id := int64(1); id <= 3; id++ {
		if !contains(report, "chat 3") {
			t.Errorf("expected chat 3 in report: %s", report)
		}
	}
}

func TestSave_MkdirAllError(t *testing.T) {
	s := New("/dev/null/stats.json")
	if err := s.Save(); err == nil {
		t.Error("expected error when MkdirAll fails")
	}
}

func TestStartPeriodicSave_TickerFires(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	s.StartPeriodicSave(ctx, 20*time.Millisecond)
}

func TestStartPeriodicSave_ContextCancel(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.StartPeriodicSave(ctx, time.Hour)
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestTrackChat(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.TrackChat(1, "vasya")
	if cs := s.PerChat[1]; cs == nil || cs.Username != "vasya" {
		t.Fatalf("expected username tracked, got %v", cs)
	}
	s.TrackChat(2, "")
	if _, ok := s.PerChat[2]; ok {
		t.Error("empty username should not create chat entry")
	}
}

func TestIncrementSuccess_UserDomains(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.TrackChat(1, "vasya")
	s.IncrementSuccess(1, "youtube.com", 100, 1000)
	s.IncrementSuccess(1, "youtube.com", 100, 1000)
	s.IncrementSuccess(1, "tiktok.com", 100, 1000)
	s.IncrementSuccess(2, "example.com", 100, 1000)

	if got := s.UserDomains["@vasya"]; len(got) != 2 {
		t.Errorf("expected 2 unique domains for @vasya, got %v", got)
	}
	if got := s.UserDomains["chat 2"]; len(got) != 1 || got[0] != "example.com" {
		t.Errorf("expected example.com for chat 2, got %v", got)
	}
}

func TestReport_Username(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.TrackChat(1, "vasya")
	s.IncrementSuccess(1, "youtube.com", 100, 1000)
	s.IncrementSuccess(2, "vimeo.com", 100, 1000)

	report := s.Report()
	if !contains(report, "@vasya: 1 / 0") {
		t.Errorf("expected @vasya in report: %s", report)
	}
	if !contains(report, "chat 2: 1 / 0") {
		t.Errorf("expected chat 2 fallback in report: %s", report)
	}
}

func TestReport_UserDomains(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	s.TrackChat(1, "vasya")
	s.IncrementSuccess(1, "youtube.com", 100, 1000)
	s.IncrementSuccess(1, "tiktok.com", 100, 1000)
	s.IncrementSuccess(1, "www.instagram.com", 100, 1000)

	report := s.Report()
	if !contains(report, "— Домены по пользователям —") {
		t.Errorf("expected user domains section: %s", report)
	}
	if !contains(report, "@vasya: instagram, tiktok, youtube") {
		t.Errorf("expected short sorted domains for @vasya: %s", report)
	}
}

func TestShortDomain(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"www.instagram.com", "instagram"},
		{"instagram.com", "instagram"},
		{"vm.tiktok.com", "tiktok"},
		{"de.pornhub.org", "pornhub"},
		{"localhost", "localhost"},
		{"127.0.0.1", "127.0.0.1"},
	}
	for _, tt := range tests {
		if got := shortDomain(tt.in); got != tt.want {
			t.Errorf("shortDomain(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestReport_UserDomainsLimits(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "stats.json"))
	for i := 0; i < 25; i++ {
		id := int64(i + 1)
		s.TrackChat(id, fmt.Sprintf("u%02d", i))
		for j := 0; j < 12; j++ {
			s.IncrementSuccess(id, fmt.Sprintf("d%02d.com", j), 100, 1000)
		}
	}

	report := s.Report()
	if !contains(report, "u00: d00, d01, d02, d03, d04, d05, d06, d07, d08, d09, …") {
		t.Errorf("expected domains truncated for u00: %s", report)
	}
	if contains(report, "\nu20:") {
		t.Errorf("expected at most 20 users in report: %s", report)
	}
}
