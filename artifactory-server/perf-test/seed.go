//go:build ignore

// Copyright 2026 The MathWorks, Inc.

// seed.go generates and publishes N realistic synthetic .mltbx packages.
// Usage: go run seed.go -count=1000 -server=http://localhost:8080 -token=<token>
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// --- Domain themes for realistic package names ---

var themes = []struct {
	Domain   string
	Prefixes []string
	Suffixes []string
}{
	{"Automotive", []string{"vehicle", "lidar", "can", "adas", "chassis", "powertrain", "ecu"}, []string{"toolkit", "analyzer", "driver", "interface", "suite"}},
	{"Manufacturing", []string{"robot", "plc", "conveyor", "quality", "assembly", "cnc"}, []string{"controller", "monitor", "toolkit", "bridge", "manager"}},
	{"DataScience", []string{"data", "stats", "ml", "feature", "pipeline", "etl"}, []string{"toolkit", "explorer", "engine", "framework", "utils"}},
	{"MachineLearning", []string{"neural", "deep", "train", "predict", "classify", "cluster"}, []string{"net", "model", "toolkit", "framework", "engine"}},
	{"ImageProcessing", []string{"image", "pixel", "filter", "segment", "morph", "color"}, []string{"toolkit", "processor", "analyzer", "utils", "lab"}},
	{"SignalProcessing", []string{"signal", "fft", "filter", "spectrum", "audio", "radar"}, []string{"toolkit", "analyzer", "processor", "utils", "suite"}},
	{"ControlSystems", []string{"pid", "servo", "plant", "feedback", "state", "lqr"}, []string{"controller", "designer", "toolkit", "tuner", "suite"}},
	{"Financial", []string{"risk", "portfolio", "option", "market", "trade", "quant"}, []string{"toolkit", "engine", "analyzer", "model", "suite"}},
	{"Geospatial", []string{"geo", "map", "terrain", "satellite", "coordinate", "gps"}, []string{"toolkit", "mapper", "analyzer", "viewer", "utils"}},
	{"Network", []string{"tcp", "packet", "route", "protocol", "socket", "http"}, []string{"toolkit", "analyzer", "monitor", "utils", "bridge"}},
	{"CloudComputing", []string{"cloud", "cluster", "deploy", "scale", "container", "batch"}, []string{"toolkit", "manager", "connector", "utils", "framework"}},
	{"Embedded", []string{"fpga", "soc", "dsp", "firmware", "mcu", "rtos"}, []string{"toolkit", "coder", "interface", "driver", "builder"}},
	{"Simulation", []string{"sim", "model", "dynamics", "physics", "monte", "solver"}, []string{"toolkit", "engine", "framework", "runner", "suite"}},
	{"Testing", []string{"test", "bench", "verify", "validate", "coverage", "harness"}, []string{"toolkit", "framework", "runner", "suite", "utils"}},
	{"Wireless", []string{"antenna", "rf", "channel", "ofdm", "mimo", "beam"}, []string{"toolkit", "designer", "analyzer", "model", "suite"}},
}

var releases = []string{">=R2024a", ">=R2024b", ">=R2025a", ">=R2025b", ">=R2026a", ">=R2026b"}

var platformSets = [][]string{
	{"win64", "maci64", "glnxa64"},
	{"win64", "maci64", "glnxa64", "maca64"},
	{"win64", "glnxa64"},
	{"win64", "maci64", "maca64", "glnxa64"},
}

var organizations = []string{
	"Acme Engineering", "DataCorp", "SignalTech", "ImageLabs",
	"ControlDynamics", "QuantAnalytics", "GeoSystems", "NetWorks",
	"CloudScale", "EmbedTech", "SimCorp", "TestForge",
	"AutoDrive", "RoboWorks", "WaveTech",
}

var firstNames = []string{
	"Alex", "Jordan", "Morgan", "Taylor", "Casey",
	"Riley", "Quinn", "Avery", "Sage", "Blake",
	"Cameron", "Drew", "Finley", "Harper", "Jamie",
}

var lastNames = []string{
	"Chen", "Smith", "Patel", "Garcia", "Kim",
	"Mueller", "Sato", "Johansson", "Okafor", "Santos",
	"Singh", "Anderson", "Thompson", "Wilson", "Lee",
}

// --- MATLAB source file templates ---

var functionTemplate = `function result = %s(input)
%%  %s - Process input data and return computed result.
%%
%%  result = %s(input) applies the %s algorithm to the input
%%  data and returns the processed output.

    arguments
        input (:,:) double
    end

    result = zeros(size(input));
    for idx = 1:numel(input)
        result(idx) = compute(input(idx));
    end
end

function y = compute(x)
    y = x .* (1 + sin(x) ./ (1 + abs(x)));
end
`

var classTemplate = `classdef %s < handle
%%  %s - Provides %s capabilities.
%%
%%  obj = %s() creates a new instance.

    properties
        Data (:,:) double = []
        Options struct = struct()
        IsInitialized (1,1) logical = false
    end

    properties (Access = private)
        Cache_
        Logger_
    end

    methods
        function obj = %s(options)
            arguments
                options.Verbose (1,1) logical = false
            end
            obj.Options = options;
            obj.Logger_ = struct('verbose', options.Verbose);
            obj.IsInitialized = true;
        end

        function result = process(obj, input)
            arguments
                obj
                input (:,:) double
            end
            obj.Data = input;
            result = obj.runPipeline(input);
            obj.Cache_ = result;
        end
    end

    methods (Access = private)
        function result = runPipeline(obj, data)
            result = data .* (1 + randn(size(data)) * 0.01);
            if obj.Options.Verbose
                fprintf('Processed %%d elements\n', numel(data));
            end
        end
    end
end
`

var scriptTemplate = `%% %s
%% Script for running %s analysis pipeline.
%%
%% This script initializes the workspace, loads configuration,
%% and executes the processing pipeline.

clear; clc;

%% Configuration
config.numIterations = 100;
config.tolerance = 1e-6;
config.outputDir = fullfile(tempdir, '%s_results');

%% Initialize
if ~isfolder(config.outputDir)
    mkdir(config.outputDir);
end

fprintf('Starting %s pipeline...\n');
startTime = tic;

%% Main processing loop
results = zeros(config.numIterations, 1);
for k = 1:config.numIterations
    results(k) = computeIteration(k, config.tolerance);
end

%% Summary
elapsed = toc(startTime);
fprintf('Completed %%d iterations in %%.2f seconds.\n', config.numIterations, elapsed);
fprintf('Mean result: %%.4f\n', mean(results));

function y = computeIteration(k, tol)
    x = randn(100, 1);
    y = sum(abs(x) > tol) / numel(x);
    y = y * log(k + 1);
end
`

// --- Package generation ---

type mpackage struct {
	Name                 string   `json:"name"`
	ID                   string   `json:"id"`
	Version              string   `json:"version"`
	DisplayName          string   `json:"displayName"`
	Summary              string   `json:"summary"`
	Provider             provider `json:"provider"`
	ReleaseCompatibility string   `json:"releaseCompatibility"`
	SupportedPlatforms   []string `json:"supportedPlatforms"`
}

type provider struct {
	Name         string `json:"name"`
	Organization string `json:"organization"`
	Email        string `json:"email"`
}

func main() {
	count := flag.Int("count", 1000, "number of packages to generate and publish")
	offset := flag.Int("offset", 0, "starting index for package generation")
	server := flag.String("server", "http://localhost:8080", "target server URL")
	token := flag.String("token", "", "bearer token for authorization (or set PERF_SERVICE_TOKEN)")
	parallel := flag.Int("parallel", 5, "number of concurrent publishers")
	flag.Parse()

	if *token == "" {
		*token = os.Getenv("PERF_SERVICE_TOKEN")
	}
	if *token == "" {
		fmt.Fprintln(os.Stderr, "ERROR: -token flag or PERF_SERVICE_TOKEN env var is required")
		os.Exit(1)
	}

	start := time.Now()
	var published atomic.Int64
	var errCount atomic.Int64

	work := make(chan int, *count)
	for i := range *count {
		work <- *offset + i
	}
	close(work)

	var wg sync.WaitGroup
	for range *parallel {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 60 * time.Second}
			for i := range work {
				err := publishPackage(client, *server, *token, i)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  ERROR publishing package %d: %v\n", i, err)
					errCount.Add(1)
					continue
				}
				n := published.Add(1)
				step := int64(*count) / 4
				if step < 1000 {
					step = int64(*count)
				}
				if n == int64(*count) || (step > 0 && step < int64(*count) && n%step == 0) {
					fmt.Printf("  Published %d/%d\n", n, *count)
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)
	fmt.Printf("  Seeding complete: %d packages in %s (%d errors)\n",
		published.Load(), elapsed.Round(time.Millisecond), errCount.Load())

	if errCount.Load() > 0 {
		os.Exit(1)
	}
}

func publishPackage(client *http.Client, server, token string, i int) error {
	// Use a deterministic RNG per package so metadata is consistent
	pkgRng := rand.New(rand.NewSource(int64(i) * 31337))
	pkg := generatePackageInfo(pkgRng, i)
	mltbx := generateMltbx(pkgRng, pkg)

	filename := fmt.Sprintf("%s-%s.mltbx", pkg.Name, pkg.Version)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(mltbx); err != nil {
		return err
	}
	writer.Close()

	req, err := http.NewRequest("POST", server+"/v1/packages/publish", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}

func generatePackageInfo(rng *rand.Rand, i int) mpackage {
	theme := themes[i%len(themes)]
	prefix := theme.Prefixes[i/len(themes)%len(theme.Prefixes)]
	suffix := theme.Suffixes[i/(len(themes)*len(theme.Prefixes))%len(theme.Suffixes)]
	name := fmt.Sprintf("%s-%s-%s", prefix, suffix, fmt.Sprintf("%04d", i))

	major := 1 + rng.Intn(3)
	minor := rng.Intn(10)
	patch := rng.Intn(20)
	version := fmt.Sprintf("%d.%d.%d", major, minor, patch)

	org := organizations[i%len(organizations)]
	firstName := firstNames[rng.Intn(len(firstNames))]
	lastName := lastNames[rng.Intn(len(lastNames))]

	return mpackage{
		Name:                 name,
		ID:                   fmt.Sprintf("%08x-%04x-4000-8000-%012x", rng.Int31(), i%65536, i),
		Version:              version,
		DisplayName:          fmt.Sprintf("%s %s %s", capitalize(prefix), capitalize(suffix), theme.Domain),
		Summary:              fmt.Sprintf("Provides %s capabilities for %s workflows.", prefix, theme.Domain),
		Provider:             provider{Name: firstName + " " + lastName, Organization: org, Email: fmt.Sprintf("%s.%s@example.com", firstName, lastName)},
		ReleaseCompatibility: releases[i%len(releases)],
		SupportedPlatforms:   platformSets[rng.Intn(len(platformSets))],
	}
}

func generateMltbx(rng *rand.Rand, pkg mpackage) []byte {
	mpackageJSON, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("marshaling package metadata: %v", err))
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// mpackage.json (required by server)
	f, err := zw.Create("fsroot/resources/mpackage.json")
	if err != nil {
		panic(fmt.Sprintf("creating zip entry: %v", err))
	}
	f.Write(mpackageJSON)

	// Generate 3-6 realistic MATLAB source files
	numFiles := 3 + rng.Intn(4)
	for j := range numFiles {
		addMatlabFile(zw, rng, pkg.Name, j)
	}

	if err := zw.Close(); err != nil {
		panic(fmt.Sprintf("closing zip writer: %v", err))
	}
	return buf.Bytes()
}

func addMatlabFile(zw *zip.Writer, rng *rand.Rand, pkgName string, idx int) {
	kind := rng.Intn(3) // 0=function, 1=class, 2=script
	funcName := fmt.Sprintf("%s_func%d", sanitize(pkgName), idx)

	var content string
	var path string

	switch kind {
	case 0:
		content = fmt.Sprintf(functionTemplate, funcName, funcName, funcName, pkgName)
		path = fmt.Sprintf("fsroot/toolbox/%s.m", funcName)
	case 1:
		className := fmt.Sprintf("%sProcessor%d", capitalize(sanitize(pkgName)), idx)
		content = fmt.Sprintf(classTemplate, className, className, pkgName, className, className)
		path = fmt.Sprintf("fsroot/toolbox/@%s/%s.m", className, className)
	case 2:
		scriptName := fmt.Sprintf("run_%s_%d", sanitize(pkgName), idx)
		content = fmt.Sprintf(scriptTemplate, scriptName, pkgName, pkgName, pkgName)
		path = fmt.Sprintf("fsroot/toolbox/%s.m", scriptName)
	}

	f, err := zw.Create(path)
	if err != nil {
		panic(fmt.Sprintf("creating zip entry %s: %v", path, err))
	}
	f.Write([]byte(content))
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := range len(s) {
		c := s[i]
		if c == '-' {
			out = append(out, '_')
		} else if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			out = append(out, c)
		}
	}
	return string(out)
}
