package tui

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"golang.design/x/clipboard"

	"twchfetch/internal/api"
	"twchfetch/internal/config"
	"twchfetch/internal/logging"
	"twchfetch/internal/player"
	"twchfetch/internal/util"
)

// activeStreamerList returns the list of streamers to refresh.
// When an OAuth token is configured the followed-channel list (fetched from
// Twitch) is used in place of the config's custom streamer list.
func (m Model) activeStreamerList() []string {
	if m.apiClient.HasAuth() && len(m.followedList) > 0 {
		return m.followedList
	}
	return m.cfg.Streamers.List
}

func (m Model) startRefresh() (Model, tea.Cmd) {
	m.isRefreshing = true
	m.progressDone = 0
	m.progressSpringPos = 0
	m.progressSpringVel = 0
	m.cache.InvalidateAll()
	// Invalidate any pending auto-refresh tick so it doesn't fire redundantly
	// right after the manual refresh completes.  RefreshDoneMsg will re-anchor
	// the countdown from the moment new data arrives.
	m.autoRefreshGen++

	// When OAuth is set, refresh the follow list first; the FollowListFetchedMsg
	// handler will kick off the actual batch status refresh.
	if m.apiClient.HasAuth() {
		logging.Info("Refresh started (OAuth) — fetching follow list first")
		m.progressCh = make(chan int, 32)
		return m, tea.Batch(func() tea.Msg { return m.spinner.Tick() }, fetchFollowListCmd(m.apiClient))
	}

	list := m.activeStreamerList()
	total := batchCount(len(list), m.cfg.RefreshBatchSize)
	m.progressTotal = total
	logging.Info("Refresh started", "streamers", len(list), "batches", total)
	m.progressCh = make(chan int, 32)
	return m, tea.Batch(
		func() tea.Msg { return m.spinner.Tick() },
		m.progressBar.SetPercent(0),
		m.doRefresh(),
		waitForProgress(m.progressCh, 0, total),
	)
}

// runBatchFetch executes a parallel batch live-status fetch.
// When progressCh is non-nil each completed batch sends one token to it and
// errors are logged; when nil the refresh is silent (errors silently dropped).
func runBatchFetch(client *api.Client, batches [][]string, maxWorkers int, progressCh chan<- int) []api.StreamerStatus {
	var mu sync.Mutex
	var allResults []api.StreamerStatus
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, batch := range batches {
		wg.Add(1)
		b := batch
		batchIdx := i + 1
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// Small random jitter (0–150 ms) after acquiring the semaphore slot.
			// Desynchronises workers that all become runnable at the same instant
			// so requests arrive at the GQL endpoint spread out rather than in
			// a single burst.
			time.Sleep(time.Duration(rand.Int63n(int64(150 * time.Millisecond))))
			results, err := client.FetchLiveStatusBatch(b)
			if progressCh != nil {
				if err != nil {
					logging.Warn("Batch fetch error", "batch", batchIdx, "err", err)
				} else {
					logging.Debug("Batch fetched", "batch", batchIdx, "results", len(results))
				}
				progressCh <- 1
			}
			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return allResults
}

func (m Model) doRefresh() tea.Cmd {
	streamers := m.activeStreamerList()
	batches := chunkStrings(streamers, m.cfg.RefreshBatchSize)
	client := m.apiClient
	progressCh := m.progressCh
	return func() tea.Msg {
		results := runBatchFetch(client, batches, m.cfg.RefreshMaxWorkers, progressCh)
		orderMap := make(map[string]int, len(streamers))
		for i, s := range streamers {
			orderMap[strings.ToLower(s)] = i
		}
		sort.Slice(results, func(i, j int) bool {
			return orderMap[strings.ToLower(results[i].Username)] <
				orderMap[strings.ToLower(results[j].Username)]
		})
		return RefreshDoneMsg{Results: results}
	}
}

func (m Model) doSilentRefresh() tea.Cmd {
	streamers := m.activeStreamerList()
	batches := chunkStrings(streamers, m.cfg.RefreshBatchSize)
	client := m.apiClient
	return func() tea.Msg {
		results := runBatchFetch(client, batches, m.cfg.RefreshMaxWorkers, nil)
		orderMap := make(map[string]int, len(streamers))
		for i, s := range streamers {
			orderMap[strings.ToLower(s)] = i
		}
		sort.Slice(results, func(i, j int) bool {
			return orderMap[strings.ToLower(results[i].Username)] <
				orderMap[strings.ToLower(results[j].Username)]
		})
		return SilentRefreshDoneMsg{Results: results}
	}
}

// fetchFollowListCmd fetches the authenticated user's followed channels.
func fetchFollowListCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		logins, err := client.FetchFollowedChannels()
		return FollowListFetchedMsg{Logins: logins, Err: err}
	}
}

// launchBatchRefresh starts the actual batch live-status refresh after the
// streamer list is known. Called from FollowListFetchedMsg handler.
// Returns the updated model (with progressTotal set) alongside the Cmd.
func (m Model) launchBatchRefresh() (Model, tea.Cmd) {
	list := m.activeStreamerList()
	total := batchCount(len(list), m.cfg.RefreshBatchSize)
	m.progressTotal = total
	return m, tea.Batch(
		m.progressBar.SetPercent(0),
		m.doRefresh(),
		waitForProgress(m.progressCh, 0, total),
	)
}

func waitForProgress(ch chan int, done, total int) tea.Cmd {
	return func() tea.Msg {
		_, ok := <-ch
		if !ok {
			return nil
		}
		return BatchProgressMsg{Completed: done + 1, Total: total}
	}
}

func fetchDetailsCmd(client *api.Client, username string) tea.Cmd {
	return func() tea.Msg {
		details, followers, _ := client.FetchLiveDetails(username)
		vods, _ := client.FetchVODs(username, 1)
		var lastSeen *time.Time
		if len(vods) > 0 {
			if t, ok := util.StreamEndTime(vods[0].CreatedAt, vods[0].LengthSeconds); ok {
				lastSeen = &t
			}
		}
		return DetailsFetchedMsg{Username: username, Details: details, LastSeen: lastSeen, Followers: followers}
	}
}

func fetchVODsCmd(client *api.Client, username string, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		count := cfg.Display.VodMaxDisplay
		vods, _ := client.FetchVODs(username, count)
		vodIDs := make([]string, len(vods))
		for i, v := range vods {
			vodIDs[i] = v.ID
		}
		chapters, _ := client.FetchVODChapters(vodIDs, cfg.ChapterHash)
		// A full page result means there may be more VODs to load.
		hasMore := len(vods) >= count
		return VODsFetchedMsg{Username: username, VODs: vods, Chapters: chapters, HasMore: hasMore}
	}
}

// fetchMoreVODsCmd fetches alreadyHave+10 VODs from the beginning, then
// returns only the new tail (indices [alreadyHave:]). This avoids cursor
// pagination, which Twitch's anonymous GQL endpoint does not reliably support.
// Chapters are fetched only for the newly retrieved VODs.
func fetchMoreVODsCmd(client *api.Client, username string, alreadyHave int, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		want := alreadyHave + 10
		allVODs, _ := client.FetchVODs(username, want)

		// Extract the new tail — the slice beyond what the model already holds.
		var newVODs []api.VOD
		if len(allVODs) > alreadyHave {
			newVODs = allVODs[alreadyHave:]
		}
		// A full-page result suggests more VODs may still exist.
		hasMore := len(allVODs) >= want

		vodIDs := make([]string, len(newVODs))
		for i, v := range newVODs {
			vodIDs[i] = v.ID
		}
		chapters, _ := client.FetchVODChapters(vodIDs, cfg.ChapterHash)
		return VODsMoreFetchedMsg{Username: username, VODs: newVODs, Chapters: chapters, HasMore: hasMore}
	}
}

func fetchLatestVODCmd(client *api.Client, username string) tea.Cmd {
	return func() tea.Msg {
		vods, err := client.FetchVODs(username, 1)
		if err != nil || len(vods) == 0 {
			if err == nil {
				err = fmt.Errorf("no VODs found")
			}
			return LatestVODFetchedMsg{Username: username, Err: err}
		}
		return LatestVODFetchedMsg{Username: username, VODID: vods[0].ID}
	}
}

func launchPlayerCmd(url string, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		return MPVLaunchedMsg{Err: player.Launch(url, cfg.PlayerPath, cfg.PlayerType, cfg.PlayerArgs)}
	}
}

// fetchEmotesCmd fetches BTTV and 7TV emote sets for the given channel.
// The result is delivered as EmotesFetchedMsg so the model can update the
// active emote set without blocking the UI.
func fetchEmotesCmd(client *api.Client, channel string) tea.Cmd {
	return func() tea.Msg {
		emotes := client.FetchEmoteSet(channel)
		return EmotesFetchedMsg{Channel: channel, Emotes: emotes}
	}
}

func copyCmd(text string, cbAvailable bool) tea.Cmd {
	return func() tea.Msg {
		if !cbAvailable {
			return ClipboardMsg{Text: text, Err: fmt.Errorf("clipboard not available")}
		}
		clipboard.Write(clipboard.FmtText, []byte(text))
		return ClipboardMsg{Text: text}
	}
}

