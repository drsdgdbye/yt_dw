package stats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrCreateStatsDir  = errors.New("create stats dir")
	ErrCreateTempFile  = errors.New("create temp file")
	ErrEncodeStats     = errors.New("encode stats")
	ErrCloseTempFile   = errors.New("close temp file")
	ErrRenameStatsFile = errors.New("rename stats file")
)

// ChatStats — статистика по одному чату: username, успешные и неуспешные обработки.
type ChatStats struct {
	Username string `json:"username,omitempty"`
	Success  int64  `json:"success"`
	Failed   int64  `json:"failed"`
}

// Stats — потокоопасная статистика бота с автосохранением в JSON.
type Stats struct {
	mu       sync.RWMutex
	savePath string

	NewChats          int64                `json:"new_chats"`
	TotalProcessed    int64                `json:"total_processed"`
	TotalSuccess      int64                `json:"total_success"`
	TotalFailed       int64                `json:"total_failed"`
	PerChat           map[int64]*ChatStats `json:"per_chat"`
	TopDomains        map[string]int64     `json:"top_domains"`
	UserDomains       map[string][]string  `json:"user_domains"`
	ErrorStats        map[string]int64     `json:"error_stats"`
	FileSizes         []int64              `json:"file_sizes"`
	ProcessingTimesMs []int64              `json:"processing_times_ms"`
}

// New создаёт Stats, загружает существующие данные из savePath.
func New(savePath string) *Stats {
	s := &Stats{
		savePath:          savePath,
		PerChat:           make(map[int64]*ChatStats),
		TopDomains:        make(map[string]int64),
		UserDomains:       make(map[string][]string),
		ErrorStats:        make(map[string]int64),
		FileSizes:         make([]int64, 0),
		ProcessingTimesMs: make([]int64, 0),
	}
	s.load()
	return s
}

// IncrementNewChats увеличивает счётчик новых чатов.
func (s *Stats) IncrementNewChats() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.NewChats++
}

// IncrementProcessed увеличивает общее количество обработанных ссылок.
func (s *Stats) IncrementProcessed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalProcessed++
}

// TrackChat запоминает username отправителя для чата.
func (s *Stats) TrackChat(chatID int64, username string) {
	if username == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.perChat(chatID).Username = username
}

// IncrementSuccess фиксирует успешную обработку: чат, домен, размер файла, время.
func (s *Stats) IncrementSuccess(chatID int64, domain string, fileSize, procTimeMs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalSuccess++

	cs := s.perChat(chatID)
	cs.Success++

	if domain != "" {
		s.TopDomains[domain]++
		s.addUserDomain(chatID, cs.Username, domain)
	}

	s.FileSizes = append(s.FileSizes, fileSize)
	s.ProcessingTimesMs = append(s.ProcessingTimesMs, procTimeMs)
}

// IncrementFailed фиксирует неуспешную обработку: чат и тип ошибки.
func (s *Stats) IncrementFailed(chatID int64, errType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalFailed++

	cs := s.perChat(chatID)
	cs.Failed++

	if errType != "" {
		s.ErrorStats[errType]++
	}
}

// perChat возвращает (и создаёт при необходимости) статистику для chatID.
func (s *Stats) perChat(chatID int64) *ChatStats {
	cs, ok := s.PerChat[chatID]
	if !ok || cs == nil {
		cs = &ChatStats{}
		s.PerChat[chatID] = cs
	}
	return cs
}

// userKey возвращает ключ пользователя для UserDomains.
func userKey(chatID int64, username string) string {
	if username != "" {
		return "@" + username
	}
	return fmt.Sprintf("chat %d", chatID)
}

// addUserDomain добавляет домен в список пользователя без дублей.
func (s *Stats) addUserDomain(chatID int64, username, domain string) {
	key := userKey(chatID, username)
	for _, d := range s.UserDomains[key] {
		if d == domain {
			return
		}
	}
	s.UserDomains[key] = append(s.UserDomains[key], domain)
}

// shortDomains возвращает уникальные имена доменов без поддоменов и зоны, по алфавиту.
func shortDomains(domains []string) []string {
	seen := make(map[string]struct{}, len(domains))
	short := make([]string, 0, len(domains))
	for _, d := range domains {
		name := shortDomain(d)
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		short = append(short, name)
	}
	sort.Strings(short)

	return short
}

// shortDomain убирает поддомены и зону: www.instagram.com -> instagram.
func shortDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 || net.ParseIP(domain) != nil {
		return domain
	}
	return parts[len(parts)-2]
}

// Report формирует текст отчёта статистики для отправки в Telegram.
func (s *Stats) Report() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var b strings.Builder

	total := s.TotalProcessed
	success := s.TotalSuccess
	failed := s.TotalFailed

	b.WriteString("📊 Статистика бота\n\n")
	b.WriteString(fmt.Sprintf("Новых чатов: %d\n", s.NewChats))
	b.WriteString(fmt.Sprintf("Обработано ссылок: %d\n", total))
	if total > 0 {
		succPct := float64(success) / float64(total) * 100
		failPct := float64(failed) / float64(total) * 100
		b.WriteString(fmt.Sprintf("Успешно: %d (%.1f%%)\n", success, succPct))
		b.WriteString(fmt.Sprintf("Неуспешно: %d (%.1f%%)\n", failed, failPct))
	}

	if len(s.PerChat) > 0 {
		b.WriteString("\n— По чатам —\n")
		ids := make([]int64, 0, len(s.PerChat))
		for id := range s.PerChat {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			cs := s.PerChat[id]
			if cs == nil {
				continue
			}
			label := fmt.Sprintf("chat %d", id)
			if cs.Username != "" {
				label = "@" + cs.Username
			}
			chatTotal := cs.Success + cs.Failed
			var pct float64
			if chatTotal > 0 {
				pct = float64(cs.Success) / float64(chatTotal) * 100
			}
			b.WriteString(fmt.Sprintf("%s: %d / %d (%.1f%%)\n", label, cs.Success, cs.Failed, pct))
		}
	}

	if len(s.TopDomains) > 0 {
		b.WriteString("\n— Топ доменов —\n")
		type kv struct {
			k string
			v int64
		}
		var sorted []kv
		for k, v := range s.TopDomains {
			sorted = append(sorted, kv{k, v})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
		limit := 10
		if len(sorted) < limit {
			limit = len(sorted)
		}
		for _, kv := range sorted[:limit] {
			b.WriteString(fmt.Sprintf("%s: %d\n", kv.k, kv.v))
		}
	}

	if len(s.UserDomains) > 0 {
		b.WriteString("\n— Домены по пользователям —\n")
		users := make([]string, 0, len(s.UserDomains))
		for u := range s.UserDomains {
			users = append(users, u)
		}
		sort.Strings(users)
		const (
			maxUsers   = 20
			maxDomains = 10
		)
		for i, u := range users {
			if i >= maxUsers {
				break
			}
			domains := shortDomains(s.UserDomains[u])
			suffix := ""
			if len(domains) > maxDomains {
				domains = domains[:maxDomains]
				suffix = ", …"
			}
			b.WriteString(fmt.Sprintf("%s: %s%s\n", u, strings.Join(domains, ", "), suffix))
		}
	}

	if len(s.ErrorStats) > 0 {
		b.WriteString("\n— Ошибки —\n")
		type errKV struct {
			k string
			v int64
		}
		var sorted []errKV
		for k, v := range s.ErrorStats {
			sorted = append(sorted, errKV{k, v})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
		for _, kv := range sorted {
			b.WriteString(fmt.Sprintf("%s: %d\n", kv.k, kv.v))
		}
	}

	if len(s.FileSizes) > 0 {
		var sum int64
		for _, sz := range s.FileSizes {
			sum += sz
		}
		avg := float64(sum) / float64(len(s.FileSizes))
		b.WriteString(fmt.Sprintf("\nСредний размер файла: %.1f MB\n", avg/1024/1024))
	}

	if len(s.ProcessingTimesMs) > 0 {
		var sum int64
		for _, t := range s.ProcessingTimesMs {
			sum += t
		}
		avg := float64(sum) / float64(len(s.ProcessingTimesMs))
		median := s.median()

		b.WriteString(fmt.Sprintf("Среднее время: %.1f сек\n", avg/1000))
		b.WriteString(fmt.Sprintf("Медианное время: %.1f сек\n", float64(median)/1000))
	}

	return b.String()
}

// median вычисляет медиану ProcessingTimesMs (без блокировки — вызывает внутри Report).
func (s *Stats) median() int64 {
	n := len(s.ProcessingTimesMs)
	if n == 0 {
		return 0
	}
	tmp := make([]int64, n)
	copy(tmp, s.ProcessingTimesMs)
	sort.Slice(tmp, func(i, j int) bool { return tmp[i] < tmp[j] })
	if n%2 == 0 {
		return (tmp[n/2-1] + tmp[n/2]) / 2
	}
	return tmp[n/2]
}

// Save сохраняет статистику в JSON (атомарная запись через tmp + rename).
func (s *Stats) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.save()
}

// save выполняет атомарную запись: создаёт директорию, пишет во временный файл, переименовывает.
func (s *Stats) save() error {
	dir := filepath.Dir(s.savePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("create stats dir", "error", err)
		return ErrCreateStatsDir
	}

	tmpPath := s.savePath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return ErrCreateTempFile
	}

	if err := json.NewEncoder(f).Encode(s); err != nil {
		f.Close()
		os.Remove(tmpPath)
		slog.Error("encode stats", "error", err)
		return ErrEncodeStats
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		slog.Error("close temp file", "error", err)
		return ErrCloseTempFile
	}

	if err := os.Rename(tmpPath, s.savePath); err != nil {
		os.Remove(tmpPath)
		slog.Error("rename stats file", "error", err)
		return ErrRenameStatsFile
	}

	return nil
}

// load читает JSON с диска и инициализирует nil-мапы.
func (s *Stats) load() {
	data, err := os.ReadFile(s.savePath)
	if err != nil {
		return
	}

	if err := json.Unmarshal(data, s); err != nil {
		return
	}

	if s.PerChat == nil {
		s.PerChat = make(map[int64]*ChatStats)
	}
	for id, cs := range s.PerChat {
		if cs == nil {
			delete(s.PerChat, id)
		}
	}
	if s.TopDomains == nil {
		s.TopDomains = make(map[string]int64)
	}
	if s.UserDomains == nil {
		s.UserDomains = make(map[string][]string)
	}
	if s.ErrorStats == nil {
		s.ErrorStats = make(map[string]int64)
	}
	if s.FileSizes == nil {
		s.FileSizes = make([]int64, 0)
	}
	if s.ProcessingTimesMs == nil {
		s.ProcessingTimesMs = make([]int64, 0)
	}
}

// StartPeriodicSave запускает фоновое сохранение по таймеру + при ctx.Done().
func (s *Stats) StartPeriodicSave(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = s.Save()
		case <-ctx.Done():
			_ = s.Save()
			return
		}
	}
}
