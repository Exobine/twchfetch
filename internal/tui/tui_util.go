package tui

import "twchfetch/internal/api"

func isDigit(s string) bool {
	return len(s) == 1 && s[0] >= '0' && s[0] <= '9'
}

func isPrintable(s string) bool {
	if len(s) != 1 {
		return false
	}
	b := s[0]
	return b >= 32 && b < 127
}

func parseNumBuf(buf string, count int) (int, bool) {
	if buf == "" {
		return 0, false
	}
	n := 0
	for _, c := range buf {
		n = n*10 + int(c-'0')
	}
	if n < 1 || n > count {
		return 0, false
	}
	return n - 1, true
}

func chunkStrings(s []string, size int) [][]string {
	var chunks [][]string
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}

func batchCount(total, batchSize int) int {
	if batchSize <= 0 {
		return 1
	}
	return (total + batchSize - 1) / batchSize
}

func countLive(streamers []api.StreamerStatus) int {
	n := 0
	for _, s := range streamers {
		if s.IsLive {
			n++
		}
	}
	return n
}
