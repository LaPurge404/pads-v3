package runner

import (
    "testing"

    "pads-v3/internal/ci/gates"
    "pads-v3/internal/ci/certification"
)

func Test_CIReport_Structure(t *testing.T) {
    gr := &GateRunner{
        Gates: []gates.Gate{},
    }

    cert := &certification.Certificate{
        RunID: "test",
        Deterministic: true,
    }

    report := gr.Run(gates.GateInput{}, cert, []string{})

    if report.RunID == "" {
        t.Fatal("missing run id")
    }

    if !report.Deterministic {
        t.Fatal("expected deterministic")
    }
}
