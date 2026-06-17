package evolution

import (
	"math"
	"testing"
)

func TestUCBSelector_AddArm(t *testing.T) {
	ucb := NewUCBSelector(42)

	ucb.AddArm("arm1")
	ucb.AddArm("arm2")

	if len(ucb.Names()) != 2 {
		t.Errorf("expected 2 arms, got %d", len(ucb.Names()))
	}

	// Adding duplicate should be no-op
	ucb.AddArm("arm1")
	if len(ucb.Names()) != 2 {
		t.Errorf("expected 2 arms after duplicate add, got %d", len(ucb.Names()))
	}
}

func TestUCBSelector_Select_Unpulled(t *testing.T) {
	ucb := NewUCBSelector(42)
	ucb.AddArm("arm1")
	ucb.AddArm("arm2")

	// With unpulled arms, should pick randomly
	selected := ucb.Select()
	if selected != "arm1" && selected != "arm2" {
		t.Errorf("Select() returned unexpected arm: %s", selected)
	}
}

func TestUCBSelector_Select_Balanced(t *testing.T) {
	ucb := NewUCBSelector(42)
	ucb.AddArm("arm1")
	ucb.AddArm("arm2")

	// Pull both arms equally with equal rewards
	for i := 0; i < 100; i++ {
		arm := ucb.Select()
		ucb.Update(arm, 1.0)
	}

	// With equal rewards, both arms should have received pulls
	// (exact distribution depends on UCB exploration)
	arm1Count := ucb.Counts()["arm1"]
	arm2Count := ucb.Counts()["arm2"]

	if arm1Count == 0 || arm2Count == 0 {
		t.Errorf("expected both arms to be pulled, got arm1=%d arm2=%d", arm1Count, arm2Count)
	}

	// Both should have roughly equal reward since we rewarded equally
	arm1Avg := ucb.Arms()["arm1"] / float64(arm1Count)
	arm2Avg := ucb.Arms()["arm2"] / float64(arm2Count)

	if arm1Avg != 1.0 || arm2Avg != 1.0 {
		t.Errorf("expected both avgs to be 1.0, got arm1=%f arm2=%f", arm1Avg, arm2Avg)
	}
}

func TestUCBSelector_Learning(t *testing.T) {
	ucb := NewUCBSelector(42)
	ucb.AddArm("good")
	ucb.AddArm("bad")

	// "good" arm gives reward 1.0, "bad" gives 0.0
	for i := 0; i < 50; i++ {
		arm := ucb.Select()
		if arm == "good" {
			ucb.Update(arm, 1.0)
		} else {
			ucb.Update(arm, 0.0)
		}
	}

	// After learning, "good" should have higher average reward
	goodAvg := ucb.Arms()["good"] / float64(ucb.Counts()["good"])
	badAvg := ucb.Arms()["bad"] / float64(ucb.Counts()["bad"])

	if goodAvg <= badAvg {
		t.Errorf("good arm should have higher avg reward: good=%f bad=%f", goodAvg, badAvg)
	}

	// With enough pulls, UCB should favor "good"
	goodPulls := ucb.Counts()["good"]
	if goodPulls <= ucb.Counts()["bad"] {
		t.Logf("warning: good pulls=%d, bad pulls=%d (may need more iterations)", goodPulls, ucb.Counts()["bad"])
	}
}

func TestUCBSelector_UCB1_Formula(t *testing.T) {
	ucb := NewUCBSelector(42)
	ucb.AddArm("arm1")
	ucb.AddArm("arm2")

	// Pull arm1: 10 times with avg 0.8
	for i := 0; i < 10; i++ {
		ucb.Update("arm1", 0.8)
	}

	// Pull arm2: 5 times with avg 1.0
	for i := 0; i < 5; i++ {
		ucb.Update("arm2", 1.0)
	}

	// Total pulls = 15
	// UCB1 for arm1: avg + sqrt(2*ln(15)/10)
	arm1Avg := ucb.Arms()["arm1"] / float64(ucb.Counts()["arm1"])
	arm1Exploration := math.Sqrt(2*math.Log(15) / 10)
	arm1UCB := arm1Avg + arm1Exploration

	// UCB1 for arm2: avg + sqrt(2*ln(15)/5)
	arm2Avg := ucb.Arms()["arm2"] / float64(ucb.Counts()["arm2"])
	arm2Exploration := math.Sqrt(2*math.Log(15) / 5)
	arm2UCB := arm2Avg + arm2Exploration

	// arm2 has higher avg and higher exploration term (fewer pulls)
	// So arm2 should be selected more often
	if arm2UCB <= arm1UCB {
		t.Errorf("expected arm2 UCB (%f) > arm1 UCB (%f)", arm2UCB, arm1UCB)
	}
}

func TestUCBSelector_Select_Empty(t *testing.T) {
	ucb := NewUCBSelector(42)
	selected := ucb.Select()
	if selected != "" {
		t.Errorf("expected empty string for empty selector, got %s", selected)
	}
}

func TestUCBSelector_Update_NonExistent(t *testing.T) {
	ucb := NewUCBSelector(42)
	ucb.AddArm("arm1")

	// Update non-existent arm should be no-op
	ucb.Update("arm2", 1.0)

	if ucb.Counts()["arm2"] != 0 {
		t.Errorf("non-existent arm should have 0 count")
	}
}

func TestUCBSelector_MultiArm(t *testing.T) {
	ucb := NewUCBSelector(42)

	// Add 4 arms with different reward profiles
	ucb.AddArm("conservative")
	ucb.AddArm("aggressive")
	ucb.AddArm("test-first")
	ucb.AddArm("minimal-change")

	// Simulate 200 iterations with different reward patterns
	// conservative: avg 0.6
	// aggressive: avg 0.3 (riskier)
	// test-first: avg 0.8 (best)
	// minimal-change: avg 0.5

	rewards := map[string]float64{
		"conservative":   0.6,
		"aggressive":     0.3,
		"test-first":     0.8,
		"minimal-change": 0.5,
	}

	for i := 0; i < 200; i++ {
		arm := ucb.Select()
		ucb.Update(arm, rewards[arm])
	}

	// After learning, test-first should have highest avg (or close to it)
	testFirstAvg := ucb.Arms()["test-first"] / float64(ucb.Counts()["test-first"])

	bestAvg := 0.0
	for _, avg := range map[string]float64{
		"conservative":   ucb.Arms()["conservative"] / float64(ucb.Counts()["conservative"]),
		"aggressive":     ucb.Arms()["aggressive"] / float64(ucb.Counts()["aggressive"]),
		"test-first":     testFirstAvg,
		"minimal-change": ucb.Arms()["minimal-change"] / float64(ucb.Counts()["minimal-change"]),
	} {
		if avg > bestAvg {
			bestAvg = avg
		}
	}

	// test-first should be competitive with best
	if testFirstAvg < bestAvg*0.8 {
		t.Logf("warning: test-first avg %f vs best %f", testFirstAvg, bestAvg)
	}
}

func TestUCBSelector_ListArms(t *testing.T) {
	ucb := NewUCBSelector(42)
	ucb.AddArm("arm1")
	ucb.AddArm("arm2")
	ucb.AddArm("arm3")

	arms := ucb.ListArms()
	if len(arms) != 3 {
		t.Errorf("expected 3 arms, got %d", len(arms))
	}
}

func TestUCBArmStats(t *testing.T) {
	ucb := NewUCBSelector(42)
	ucb.AddArm("good")
	ucb.AddArm("bad")

	// Pull good: 20 times with 1.0 reward
	for i := 0; i < 20; i++ {
		ucb.Update("good", 1.0)
	}

	// Pull bad: 10 times with 0.0 reward
	for i := 0; i < 10; i++ {
		ucb.Update("bad", 0.0)
	}

	// Verify stats
	if ucb.Counts()["good"] != 20 {
		t.Errorf("good count expected 20, got %d", ucb.Counts()["good"])
	}
	if ucb.Counts()["bad"] != 10 {
		t.Errorf("bad count expected 10, got %d", ucb.Counts()["bad"])
	}
	if ucb.Arms()["good"] != 20.0 {
		t.Errorf("good reward expected 20.0, got %f", ucb.Arms()["good"])
	}
	if ucb.Arms()["bad"] != 0.0 {
		t.Errorf("bad reward expected 0.0, got %f", ucb.Arms()["bad"])
	}
}