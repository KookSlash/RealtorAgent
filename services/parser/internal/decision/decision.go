package decision

func ShouldSkipProcessed(processed bool) bool {
	return processed
}

func ShouldProcessAttempt(attemptCount, maxRetries int) bool {
	return attemptCount < maxRetries
}

func ShouldInsertHistory(latestHash, currentHash string) bool {
	return latestHash == "" || latestHash != currentHash
}
