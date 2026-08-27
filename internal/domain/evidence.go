package domain

import "sort"

func AllTasksClosed(tasks []ResamplingTask) bool {
	for _, t := range tasks {
		if t.Status != ResamplingClosed {
			return false
		}
	}
	return true
}
func OrderedTaskIDs(tasks []ResamplingTask) []string {
	out := make([]string, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.TaskID)
	}
	sort.Strings(out)
	return out
}
func IsTerminal(status BatchStatus) bool { return status == StatusFrozen }
