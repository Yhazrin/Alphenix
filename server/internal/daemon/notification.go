package daemon

import (
	"regexp"
	"strconv"
	"strings"
)

// NotificationPriority controls flush ordering.
// Inspired by Claude Code's priority queue: immediate-preempt > fold > invalidation.
type NotificationPriority int

const (
	PriorityLow    NotificationPriority = iota // text/thinking — batched, folded
	PriorityNormal                             // tool results — flushed on tick
	PriorityHigh                               // errors — flushed immediately
	PriorityImmediate                          // progress updates — flushed immediately, preempts batch
)

// ProgressPattern tracks structured progress signals from agent output.
type ProgressPattern struct {
	Current int
	Total   int
	Phase   string // "plan", "progress", "done"
	Summary string
}

var (
	reProgress = regexp.MustCompile(`(?i)^PROGRESS:\s*step\s+(\d+)/(\d+)\s*[—–-]\s*(.+)`)
	rePlan     = regexp.MustCompile(`(?mi)^PLAN:\s*$`)
	rePlanItem = regexp.MustCompile(`(?m)^\s*(\d+)\.\s+(.+)`)
	reDone     = regexp.MustCompile(`(?i)^DONE:\s*$`)
	rePhase    = regexp.MustCompile(`(?i)^(Research|Plan|Implement|Verify)\s*[:—–]\s*(.+)`)
)

// DetectProgress extracts structured progress from agent text output.
// Returns nil if no pattern matches.
func DetectProgress(text string) *ProgressPattern {
	s := strings.TrimSpace(text)
	if s == "" {
		return nil
	}

	// PROGRESS: step 1/3 — description
	if m := reProgress.FindStringSubmatch(s); m != nil {
		current, _ := strconv.Atoi(m[1])
		total, _ := strconv.Atoi(m[2])
		return &ProgressPattern{
			Current: current,
			Total:   total,
			Phase:   "progress",
			Summary: strings.TrimSpace(m[3]),
		}
	}

	// Phase markers: "Research: ...", "Verify: ..."
	if m := rePhase.FindStringSubmatch(s); m != nil {
		return &ProgressPattern{
			Phase:   strings.ToLower(m[1]),
			Summary: strings.TrimSpace(m[2]),
		}
	}

	// PLAN: header — count items in following text
	if rePlan.MatchString(s) {
		items := rePlanItem.FindAllString(s, -1)
		return &ProgressPattern{
			Phase:   "plan",
			Total:   len(items),
			Summary: "plan defined",
		}
	}

	// DONE: header
	if reDone.MatchString(s) {
		return &ProgressPattern{
			Phase:   "done",
			Summary: "task completed",
		}
	}

	return nil
}

// MessageClass categorizes messages for priority-based flushing.
type MessageClass int

const (
	ClassBatched  MessageClass = iota // text, thinking — accumulate and fold
	ClassNormal                       // tool_use, tool_result — flush on tick
	ClassUrgent                       // error, progress — flush immediately
)

// ClassifyMessage returns the message class for priority-based flushing.
func ClassifyMessage(msgType string) MessageClass {
	switch msgType {
	case "error":
		return ClassUrgent
	case "tool_use", "tool_result":
		return ClassNormal
	default:
		return ClassBatched
	}
}
