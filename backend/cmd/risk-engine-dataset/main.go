package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"ikik-api/internal/riskengine"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: risk-engine-dataset <prepare|merge|label|suggestion-baseline|validate|readiness|evaluate>")
	}
	switch os.Args[1] {
	case "prepare":
		runPrepare(os.Args[2:])
	case "merge":
		runMerge(os.Args[2:])
	case "label":
		runLabel(os.Args[2:])
	case "suggestion-baseline":
		runSuggestionBaseline(os.Args[2:])
	case "validate":
		runValidate(os.Args[2:])
	case "readiness":
		runReadiness(os.Args[2:])
	case "evaluate":
		runEvaluate(os.Args[2:])
	default:
		fatalf("unknown command %q", os.Args[1])
	}
}

func runReadiness(args []string) {
	flags := flag.NewFlagSet("readiness", flag.ExitOnError)
	datasetPath := flags.String("dataset", "", "labeled dataset JSONL path")
	_ = flags.Parse(args)
	if *datasetPath == "" {
		fatalf("-dataset is required")
	}
	input := openInput(*datasetPath)
	defer func() { _ = input.Close() }()
	items, err := riskengine.ReadDatasetJSONL(input)
	if err != nil {
		fatalf("read dataset: %v", err)
	}
	report := riskengine.AssessTrainingReadiness(items, riskengine.DefaultTrainingGate())
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fatalf("write readiness report: %v", err)
	}
}

func runLabel(args []string) {
	reviewPath, labelsPath, outputPath := datasetTransformFlags("label", args)
	review, assignments := readReviewAndLabels(reviewPath, labelsPath)
	dataset, err := riskengine.ApplyReviewLabels(review, assignments)
	if err != nil {
		fatalf("apply review labels: %v", err)
	}
	output := openOutput(outputPath)
	defer func() { _ = output.Close() }()
	if err := riskengine.WriteDatasetJSONL(output, dataset); err != nil {
		fatalf("write labeled dataset: %v", err)
	}
	fmt.Fprintf(os.Stderr, "labeled %d review items\n", len(dataset))
}

func runSuggestionBaseline(args []string) {
	reviewPath, labelsPath, outputPath := datasetTransformFlags("suggestion-baseline", args)
	review, assignments := readReviewAndLabels(reviewPath, labelsPath)
	predictions, err := riskengine.SuggestionBaselinePredictions(review, assignments)
	if err != nil {
		fatalf("build suggestion baseline: %v", err)
	}
	output := openOutput(outputPath)
	defer func() { _ = output.Close() }()
	if err := riskengine.WritePredictionsJSONL(output, predictions); err != nil {
		fatalf("write baseline predictions: %v", err)
	}
	fmt.Fprintf(os.Stderr, "built %d baseline predictions\n", len(predictions))
}

func datasetTransformFlags(name string, args []string) (string, string, string) {
	flags := flag.NewFlagSet(name, flag.ExitOnError)
	reviewPath := flags.String("review", "", "review queue JSONL path")
	labelsPath := flags.String("labels", "", "label assignments JSONL path")
	outputPath := flags.String("output", "-", "output JSONL path or - for stdout")
	_ = flags.Parse(args)
	if *reviewPath == "" || *labelsPath == "" {
		fatalf("-review and -labels are required")
	}
	return *reviewPath, *labelsPath, *outputPath
}

func readReviewAndLabels(reviewPath, labelsPath string) ([]riskengine.ReviewItem, []riskengine.LabelAssignment) {
	reviewReader := openInput(reviewPath)
	review, err := riskengine.ReadReviewQueueJSONL(reviewReader)
	_ = reviewReader.Close()
	if err != nil {
		fatalf("read review queue: %v", err)
	}
	labelsReader := openInput(labelsPath)
	assignments, err := riskengine.ReadLabelAssignmentsJSONL(labelsReader)
	_ = labelsReader.Close()
	if err != nil {
		fatalf("read label assignments: %v", err)
	}
	return review, assignments
}

func runMerge(args []string) {
	flags := flag.NewFlagSet("merge", flag.ExitOnError)
	outputPath := flags.String("output", "-", "merged review JSONL output path or - for stdout")
	_ = flags.Parse(args)
	if flags.NArg() == 0 {
		fatalf("merge requires at least one review JSONL input")
	}
	queues := make([][]riskengine.ReviewItem, 0, flags.NArg())
	for _, path := range flags.Args() {
		input := openInput(path)
		items, err := riskengine.ReadReviewQueueJSONL(input)
		_ = input.Close()
		if err != nil {
			fatalf("read review queue %q: %v", path, err)
		}
		queues = append(queues, items)
	}
	items := riskengine.MergeReviewQueues(queues...)
	output := openOutput(*outputPath)
	defer func() { _ = output.Close() }()
	if err := riskengine.WriteReviewQueueJSONL(output, items); err != nil {
		fatalf("write merged review queue: %v", err)
	}
	fmt.Fprintf(os.Stderr, "merged %d unique review items\n", len(items))
}

func runPrepare(args []string) {
	flags := flag.NewFlagSet("prepare", flag.ExitOnError)
	inputPath := flags.String("input", "-", "raw JSONL input path or - for stdin")
	outputPath := flags.String("output", "-", "review JSONL output path or - for stdout")
	_ = flags.Parse(args)
	input := openInput(*inputPath)
	defer func() { _ = input.Close() }()
	items, err := riskengine.PrepareReviewQueue(input)
	if err != nil {
		fatalf("prepare review queue: %v", err)
	}
	output := openOutput(*outputPath)
	defer func() { _ = output.Close() }()
	if err := riskengine.WriteReviewQueueJSONL(output, items); err != nil {
		fatalf("write review queue: %v", err)
	}
	fmt.Fprintf(os.Stderr, "prepared %d unique review items\n", len(items))
}

func runValidate(args []string) {
	flags := flag.NewFlagSet("validate", flag.ExitOnError)
	datasetPath := flags.String("dataset", "", "labeled dataset JSONL path")
	_ = flags.Parse(args)
	if *datasetPath == "" {
		fatalf("-dataset is required")
	}
	input := openInput(*datasetPath)
	defer func() { _ = input.Close() }()
	items, err := riskengine.ReadDatasetJSONL(input)
	if err != nil {
		fatalf("validate dataset: %v", err)
	}
	fmt.Printf("valid dataset: %d examples\n", len(items))
}

func runEvaluate(args []string) {
	flags := flag.NewFlagSet("evaluate", flag.ExitOnError)
	datasetPath := flags.String("dataset", "", "labeled dataset JSONL path")
	predictionPath := flags.String("predictions", "", "prediction JSONL path")
	_ = flags.Parse(args)
	if *datasetPath == "" || *predictionPath == "" {
		fatalf("-dataset and -predictions are required")
	}
	goldReader := openInput(*datasetPath)
	defer func() { _ = goldReader.Close() }()
	predictionReader := openInput(*predictionPath)
	defer func() { _ = predictionReader.Close() }()
	gold, err := riskengine.ReadDatasetJSONL(goldReader)
	if err != nil {
		fatalf("read dataset: %v", err)
	}
	predictions, err := riskengine.ReadPredictionsJSONL(predictionReader)
	if err != nil {
		fatalf("read predictions: %v", err)
	}
	report := riskengine.EvaluateDataset(gold, predictions, riskengine.DefaultReadinessGate())
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fatalf("write report: %v", err)
	}
}

func openInput(path string) io.ReadCloser {
	if path == "-" {
		return io.NopCloser(os.Stdin)
	}
	file, err := os.Open(path)
	if err != nil {
		fatalf("open input %q: %v", path, err)
	}
	return file
}

func openOutput(path string) io.WriteCloser {
	if path == "-" {
		return nopWriteCloser{Writer: os.Stdout}
	}
	file, err := os.Create(path)
	if err != nil {
		fatalf("open output %q: %v", path, err)
	}
	return file
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
