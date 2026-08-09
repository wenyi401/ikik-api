package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"ikik-api/internal/riskengine"
)

func main() {
	endpoint := flag.String("endpoint", "", "OpenAI-compatible base URL or chat completions endpoint")
	model := flag.String("model", "", "adjudicator model name")
	datasetPath := flag.String("dataset", "", "labeled dataset JSONL path")
	outputPath := flag.String("output", "", "prediction JSONL output path")
	timeout := flag.Duration("timeout", 30*time.Second, "timeout for each model request")
	flag.Parse()
	if *endpoint == "" || *model == "" || *datasetPath == "" || *outputPath == "" {
		fatalf("-endpoint, -model, -dataset, and -output are required")
	}
	datasetFile, err := os.Open(*datasetPath)
	if err != nil {
		fatalf("open dataset: %v", err)
	}
	examples, err := riskengine.ReadDatasetJSONL(datasetFile)
	_ = datasetFile.Close()
	if err != nil {
		fatalf("read dataset: %v", err)
	}
	client := &http.Client{Timeout: *timeout}
	adjudicator, err := riskengine.NewOpenAIAdjudicator(riskengine.OpenAIAdjudicatorConfig{
		Endpoint: *endpoint, Model: *model, Token: strings.TrimSpace(os.Getenv("RISK_ENGINE_TOKEN")),
	}, client)
	if err != nil {
		fatalf("configure adjudicator: %v", err)
	}
	engine, err := riskengine.NewEngine(riskengine.AllTrafficDetector{}, adjudicator, riskengine.DefaultShadowPolicy())
	if err != nil {
		fatalf("configure engine: %v", err)
	}
	output, err := os.Create(*outputPath)
	if err != nil {
		fatalf("create output: %v", err)
	}
	defer func() { _ = output.Close() }()
	writer := bufio.NewWriter(output)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	for index, example := range examples {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		event, evaluateErr := engine.Evaluate(ctx, riskengine.Input{RequestID: example.ID, Text: example.Text})
		cancel()
		if evaluateErr != nil {
			fatalf("predict %s: %v", example.ID, evaluateErr)
		}
		if err := encoder.Encode(riskengine.Prediction{
			ID: example.ID, Adjudication: event.Adjudication, Recommendation: event.Outcome.Recommendation,
		}); err != nil {
			fatalf("write prediction: %v", err)
		}
		fmt.Fprintf(os.Stderr, "predicted %d/%d\r", index+1, len(examples))
	}
	if err := writer.Flush(); err != nil {
		fatalf("flush predictions: %v", err)
	}
	fmt.Fprintln(os.Stderr)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
