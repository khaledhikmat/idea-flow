package workflows

import (
	"sort"
	"testing"
)

func TestLeaderboardIncludesSubmitterAndScore(t *testing.T) {
	ideaScores := map[IdeaScoreInfoSignal]int{
		{IdeaID: "idea-2", Submitter: "alice", Score: 40}: 40,
		{IdeaID: "idea-1", Submitter: "bob", Score: 80}:   80,
	}

	leaderboard := make([]IdeaScoreInfoSignal, 0, len(ideaScores))
	for idea := range ideaScores {
		leaderboard = append(leaderboard, idea)
	}

	sort.Slice(leaderboard, func(i, j int) bool {
		return ideaScores[leaderboard[i]] > ideaScores[leaderboard[j]]
	})

	if len(leaderboard) != 2 {
		t.Fatalf("expected 2 leaderboard entries, got %d", len(leaderboard))
	}
	if leaderboard[0].IdeaID != "idea-1" || leaderboard[0].Submitter != "bob" || leaderboard[0].Score != 80 {
		t.Fatalf("expected first leaderboard entry to be idea-1 by bob with score 80, got %+v", leaderboard[0])
	}
	if leaderboard[1].IdeaID != "idea-2" || leaderboard[1].Submitter != "alice" || leaderboard[1].Score != 40 {
		t.Fatalf("expected second leaderboard entry to be idea-2 by alice with score 40, got %+v", leaderboard[1])
	}
}
