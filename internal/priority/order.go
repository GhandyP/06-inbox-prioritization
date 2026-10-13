package priority

import (
	"cmp"
	"slices"
	"time"
)

// Order returns results in deterministic priority order.
//
// The sort key is, in order: whether the result names a message in the batch,
// then the label rank, then that message's ReceivedAt, then the message ID.
//
// Every part of that key comes either from the closed label set or from the
// reader, so the position of a result never depends on the model and never
// depends on the order the results arrived in: reordering the same batch twice
// agrees. This replaces the score the earlier design proposed, because a
// model-generated score would not be reproducible.
//
// Callers are expected to run Validate first. Order stays total when they have
// not: a result whose label is unknown sorts after every known label, and a
// result whose message is absent sorts after every result that has one. Among
// themselves those stragglers order by message ID.
//
// The input slice is not modified.
func Order(results []Result, messages []Message) []Result {
	stamps := make(map[string]time.Time, len(messages))
	for _, message := range messages {
		stamps[message.ID] = message.ReceivedAt
	}

	type keyed struct {
		key    orderKey
		result Result
	}

	ordered := make([]keyed, 0, len(results))
	for _, result := range results {
		stamp, hasMessage := stamps[result.MessageID]
		ordered = append(ordered, keyed{
			key: orderKey{
				hasMessage: hasMessage,
				labelRank:  sortRank(result.Label),
				receivedAt: stamp,
				messageID:  result.MessageID,
			},
			result: result,
		})
	}

	slices.SortStableFunc(ordered, func(a, b keyed) int {
		return compareKeys(a.key, b.key)
	})

	out := make([]Result, 0, len(ordered))
	for _, item := range ordered {
		out = append(out, item.result)
	}
	return out
}

type orderKey struct {
	hasMessage bool
	labelRank  int
	receivedAt time.Time
	messageID  string
}

func compareKeys(a, b orderKey) int {
	// A result with no message has no timestamp to order by, so it goes last.
	if a.hasMessage != b.hasMessage {
		if a.hasMessage {
			return -1
		}
		return 1
	}
	if c := cmp.Compare(a.labelRank, b.labelRank); c != 0 {
		return c
	}
	if c := a.receivedAt.Compare(b.receivedAt); c != 0 {
		return c
	}
	return cmp.Compare(a.messageID, b.messageID)
}

// sortRank places an unknown label after every known label, which is the
// opposite of what Rank's -1 sentinel would do if it were used directly.
func sortRank(label Label) int {
	if rank := label.Rank(); rank >= 0 {
		return rank
	}
	return len(Labels())
}
