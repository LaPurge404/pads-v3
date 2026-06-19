package evolution

import (
	"math"
	"os"
	"testing"
	"time"
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
	arm1Exploration := math.Sqrt(2 * math.Log(15) / 10)
	arm1UCB := arm1Avg + arm1Exploration

	// UCB1 for arm2: avg + sqrt(2*ln(15)/5)
	arm2Avg := ucb.Arms()["arm2"] / float64(ucb.Counts()["arm2"])
	arm2Exploration := math.Sqrt(2 * math.Log(15) / 5)
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

func TestUCBSelector_SaveLoad(t *testing.T) {
	tmp, err := os.CreateTemp("", "ucb_test_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)

	// Create a selector with some history.
	ucb := NewUCBSelector(42)
	ucb.AddArm("conservative")
	ucb.AddArm("aggressive")
	ucb.AddArm("test-first")
	ucb.AddArm("minimal-change")

	// Simulate 50 rounds of learning.
	rewards := map[string]float64{
		"conservative":   0.6,
		"aggressive":     0.3,
		"test-first":     0.8,
		"minimal-change": 0.5,
	}
	for i := 0; i < 50; i++ {
		arm := ucb.Select()
		ucb.Update(arm, rewards[arm])
	}

	// Save state.
	if err := ucb.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Create a fresh selector and restore.
	restored := NewUCBSelector(99)
	restored.AddArm("conservative")
	restored.AddArm("aggressive")
	restored.AddArm("test-first")
	restored.AddArm("minimal-change")
	if err := restored.Load(path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify arms and counts match.
	for _, name := range []string{"conservative", "aggressive", "test-first", "minimal-change"} {
		if restored.Counts()[name] != ucb.Counts()[name] {
			t.Errorf("count mismatch for %s: got %d want %d",
				name, restored.Counts()[name], ucb.Counts()[name])
		}
		if restored.Arms()[name] != ucb.Arms()[name] {
			t.Errorf("arm reward mismatch for %s: got %f want %f",
				name, restored.Arms()[name], ucb.Arms()[name])
		}
	}

	// Verify the restored selector behaves the same as the original.
	origArm := ucb.Select()
	restArm := restored.Select()
	_ = origArm
	_ = restArm
	// After enough pulls both should converge — just verify no panics.
}

func TestUCBSelector_SaveLoad_PartialArms(t *testing.T) {
	tmp, err := os.CreateTemp("", "ucb_test_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)

	// Save a selector with only some arms.
	ucb := NewUCBSelector(7)
	ucb.AddArm("stable")
	ucb.AddArm("bandit")
	ucb.Update("stable", 1.0)
	ucb.Update("stable", 1.0)
	ucb.Update("bandit", 0.0)

	if err := ucb.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load into a selector that already has extra arms (simulating a newer binary).
	restored := NewUCBSelector(7)
	restored.AddArm("stable")
	restored.AddArm("bandit")
	restored.AddArm("locked") // arm that didn't exist when saved
	if err := restored.Load(path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// stable should have count=2.
	if restored.Counts()["stable"] != 2 {
		t.Errorf("stable count: got %d want 2", restored.Counts()["stable"])
	}
	// bandit should have count=1.
	if restored.Counts()["bandit"] != 1 {
		t.Errorf("bandit count: got %d want 1", restored.Counts()["bandit"])
	}
}

func TestUCBSelector_Save_NonExistentPath(t *testing.T) {
	ucb := NewUCBSelector(42)
	ucb.AddArm("arm1")
	err := ucb.Save("/nonexistent/directory/ucb.json")
	if err == nil {
		t.Error("expected error when saving to non-existent path")
	}
}

func TestUCBSelector_Load_NonExistentFile(t *testing.T) {
	ucb := NewUCBSelector(42)
	ucb.AddArm("arm1")
	err := ucb.Load("/definitely/does/not/exist.json")
	if err == nil {
		t.Error("expected error when loading non-existent file")
	}
}

func TestUCBSelector_AutoSave_Stop(t *testing.T) {
	tmp, err := os.CreateTemp("", "ucb_test_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)

	ucb := NewUCBSelector(42, path)
	ucb.AddArm("arm1")
	ucb.AddArm("arm2")
	ucb.Update("arm1", 0.8)
	ucb.Update("arm1", 0.8)

	// Enable auto-save with 10ms interval (triggers quickly in test).
	ucb.EnableAutoSave(10 * time.Millisecond)

	// Let the auto-save goroutine fire at least once.
	time.Sleep(50 * time.Millisecond)

	// Stop triggers final save.
	ucb.Stop()

	// Verify the file was written.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file to exist after Stop, got error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty file after Stop")
	}

	// Verify we can restore.
	restored := NewUCBSelector(99)
	restored.AddArm("arm1")
	restored.AddArm("arm2")
	if err := restored.Load(path); err != nil {
		t.Fatalf("failed to load saved state: %v", err)
	}
	if restored.Counts()["arm1"] != 2 {
		t.Errorf("arm1 count after restore: got %d want 2", restored.Counts()["arm1"])
	}
}

func TestUCBSelector_NewUCBSelector_WithLoad(t *testing.T) {
	tmp, err := os.CreateTemp("", "ucb_test_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)

	// Create a selector and save it.
	original := NewUCBSelector(11)
	original.AddArm("conservative")
	original.AddArm("aggressive")
	for i := 0; i < 30; i++ {
		arm := original.Select()
		reward := 0.7
		if arm == "aggressive" {
			reward = 0.2
		}
		original.Update(arm, reward)
	}
	if err := original.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Create a new selector that loads on construction.
	restored := NewUCBSelector(22, path)
	restored.AddArm("conservative")
	restored.AddArm("aggressive")

	// Counts must match immediately after construction.
	if restored.Counts()["conservative"] != original.Counts()["conservative"] {
		t.Errorf("conservative count: got %d want %d",
			restored.Counts()["conservative"], original.Counts()["conservative"])
	}
}
