package codeanalysis

import (
    "bytes"
    "fmt"
    "os/exec"
    "strconv"
    "strings"
)

type Result struct {
    TestsPassed  int
    TestsFailed  int
    TestsAllPass bool
    LintIssues   int
    Coverage     float64
    Score        float64
}

func Analyze() (Result, error) {
    var res Result

    // 1. Tests
    cmd := exec.Command("go", "test", "./...", "-count=1")
    testOut, testErr := cmd.CombinedOutput()
    lines := strings.Split(string(testOut), "\n")
    for _, l := range lines {
        trimmed := strings.TrimSpace(l)
        if strings.HasPrefix(trimmed, "ok ") {
            res.TestsPassed++
        } else if strings.HasPrefix(trimmed, "FAIL ") {
            res.TestsFailed++
        }
    }
    // Si la commande a échoué globalement et qu'aucun FAIL n'a été compté, on force un échec
    if testErr != nil {
        if res.TestsFailed == 0 {
            res.TestsFailed = 1
        }
    }
    res.TestsAllPass = res.TestsFailed == 0

    // 2. Lint
    lintCmd := exec.Command("golangci-lint", "run", "--out-format=line-number")
    var lintOut bytes.Buffer
    lintCmd.Stdout = &lintOut
    lintCmd.Stderr = &lintOut
    _ = lintCmd.Run()
    res.LintIssues = bytes.Count(lintOut.Bytes(), []byte("\n"))

    // 3. Couverture
    covCmd := exec.Command("go", "test", "./...", "-cover")
    var covOut bytes.Buffer
    covCmd.Stdout = &covOut
    covCmd.Stderr = &covOut
    _ = covCmd.Run()
    res.Coverage = extractCoverage(string(covOut.Bytes()))

    // 4. Score composite
    testScore := 0.0
    if res.TestsPassed+res.TestsFailed > 0 {
        testScore = float64(res.TestsPassed) / float64(res.TestsPassed+res.TestsFailed) * 100.0
    }
    lintScore := 0.0
    if res.LintIssues <= 5 {
        lintScore = 100.0
    } else if res.LintIssues <= 20 {
        lintScore = 60.0
    } else {
        lintScore = 20.0
    }
    covScore := res.Coverage
    res.Score = testScore*0.4 + lintScore*0.3 + covScore*0.3

    return res, nil
}

func (r Result) Summary() string {
    return fmt.Sprintf("Tests: %d/%d passés, Lint: %d problèmes, Couverture: %.1f%%, Score: %.1f",
        r.TestsPassed, r.TestsPassed+r.TestsFailed, r.LintIssues, r.Coverage, r.Score)
}

func extractCoverage(output string) float64 {
    lines := strings.Split(output, "\n")
    for _, l := range lines {
        if strings.Contains(l, "coverage:") {
            parts := strings.Split(l, "coverage:")
            if len(parts) < 2 {
                continue
            }
            right := strings.TrimSpace(parts[1])
            right = strings.TrimSuffix(right, "% of statements")
            right = strings.TrimSuffix(right, "%")
            val, err := strconv.ParseFloat(right, 64)
            if err == nil {
                return val
            }
        }
    }
    return 0.0
}
