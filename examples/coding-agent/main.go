// coding-agent processes code tasks: PR reviews, bug fixes, and feature
// implementations. Each task runs in a dedicated container with an isolated
// clone of the target repository mounted at /workspace/repo.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/adk/tool/mcptoolset"
	"google.golang.org/genai"
)

const repoPath = "/workspace/repo"

// CodeTask is the event payload for code-task events (schemas/code-task.json).
type CodeTask struct {
	TaskID       string   `json:"task_id"`
	Type         string   `json:"type"` // pr-review | bug-fix | feature | refactor
	Repository   string   `json:"repository"`
	Branch       string   `json:"branch"`
	PRNumber     int      `json:"pr_number,omitempty"`
	Description  string   `json:"description"`
	ContextFiles []string `json:"context_files,omitempty"`
	CreatedAt    string   `json:"created_at"`
}

// --- Tools ---

type ReadFileArgs struct {
	Path string `json:"path" jsonschema:"description=File path relative to the repository root"`
}
type ReadFileResult struct {
	Content string `json:"content"`
}

func readFile(_ tool.Context, args ReadFileArgs) (ReadFileResult, error) {
	b, err := os.ReadFile(repoPath + "/" + args.Path)
	if err != nil {
		return ReadFileResult{}, fmt.Errorf("reading %s: %w", args.Path, err)
	}
	return ReadFileResult{Content: string(b)}, nil
}

type WriteFileArgs struct {
	Path    string `json:"path"    jsonschema:"description=File path relative to the repository root"`
	Content string `json:"content" jsonschema:"description=Complete file content to write"`
}

func writeFile(_ tool.Context, args WriteFileArgs) (map[string]any, error) {
	if err := os.WriteFile(repoPath+"/"+args.Path, []byte(args.Content), 0o644); err != nil {
		return nil, fmt.Errorf("writing %s: %w", args.Path, err)
	}
	return map[string]any{"written": args.Path}, nil
}

type SearchCodeArgs struct {
	Pattern string `json:"pattern" jsonschema:"description=Regex pattern to search across Go source files"`
	Dir     string `json:"dir"     jsonschema:"description=Subdirectory to limit the search (empty = whole repo)"`
}
type SearchCodeResult struct {
	Matches []string `json:"matches"`
	Count   int      `json:"count"`
}

func searchCode(_ tool.Context, args SearchCodeArgs) (SearchCodeResult, error) {
	dir := repoPath
	if args.Dir != "" {
		dir = repoPath + "/" + args.Dir
	}
	out, _ := exec.Command("grep", "-rn", "--include=*.go", args.Pattern, dir).Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	return SearchCodeResult{Matches: lines, Count: len(lines)}, nil
}

type RunTestsArgs struct {
	Pattern string `json:"pattern" jsonschema:"description=Go test pattern (e.g. ./... or ./pkg/payments/...)"`
}
type RunTestsResult struct {
	Output string `json:"output"`
	Passed bool   `json:"passed"`
}

func runTests(_ tool.Context, args RunTestsArgs) (RunTestsResult, error) {
	cmd := exec.Command("go", "test", "-v", args.Pattern)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return RunTestsResult{Output: string(out), Passed: err == nil}, nil
}

// --- Agent ---

func buildAgent(ctx context.Context) (agent.Agent, error) {
	// Orchestrator injects the inference endpoint via labels:
	//   org.openagentcontainers.inference.api_base.env="OPENAI_BASE_URL"
	//   org.openagentcontainers.inference.api_key.env="OPENAI_API_KEY"
	model, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})
	if err != nil {
		return nil, fmt.Errorf("creating model: %w", err)
	}

	readFileTool, _ := functiontool.New(functiontool.Config{
		Name:        "read_file",
		Description: "Read a source file from the repository workspace.",
	}, readFile)

	writeFileTool, _ := functiontool.New(functiontool.Config{
		Name:        "write_file",
		Description: "Write or overwrite a file in the repository workspace.",
	}, writeFile)

	searchCodeTool, _ := functiontool.New(functiontool.Config{
		Name:        "search_code",
		Description: "Search for a regex pattern across Go source files in the repository.",
	}, searchCode)

	runTestsTool, _ := functiontool.New(functiontool.Config{
		Name:        "run_tests",
		Description: "Run Go tests and return pass/fail status with full output.",
	}, runTests)

	// GitHub MCP server — credentials injected via DCR (spec §5.3).
	githubToolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.CommandTransport{
			Command: exec.Command("github-mcp-server",
				"--client-id", os.Getenv("GITHUB_CLIENT_ID"),
				"--client-secret", os.Getenv("GITHUB_CLIENT_SECRET"),
			),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating github toolset: %w", err)
	}

	// CI/CD MCP server — bearer token injected by orchestrator (spec §5.3).
	ciToolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.CommandTransport{
			Command: exec.Command("ci-mcp-server", "--token", os.Getenv("CI_TOKEN")),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating ci toolset: %w", err)
	}

	return llmagent.New(llmagent.Config{
		Name:  "coding-agent",
		Model: model,
		Instruction: `You are a senior software engineer completing code tasks in a Go repository
mounted at /workspace/repo.

For pr-review tasks: read every changed file with read_file, check for bugs,
missing error handling, test coverage gaps, and style issues. Post a detailed
review with inline comments using the GitHub tools.

For bug-fix, feature, and refactor tasks: understand the codebase with
search_code and read_file, make targeted changes with write_file, then run
tests with run_tests to verify correctness. Open a pull request via GitHub
tools when done.

Never declare a task complete until tests pass.`,
		Tools:    []tool.Tool{readFileTool, writeFileTool, searchCodeTool, runTestsTool},
		Toolsets: []tool.Toolset{githubToolset, ciToolset},
	})
}

func main() {
	ctx := context.Background()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	ag, err := buildAgent(ctx)
	if err != nil {
		log.Error("building agent", "error", err)
		os.Exit(1)
	}

	r, err := runner.New(runner.Config{
		AppName:           "coding-agent",
		Agent:             ag,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		log.Error("creating runner", "error", err)
		os.Exit(1)
	}

	runHarness(ctx, func(ctx context.Context, evt OACEvent) error {
		var task CodeTask
		if err := json.Unmarshal(evt.Payload, &task); err != nil {
			return fmt.Errorf("decoding code-task event: %w", err)
		}

		log.Info("starting task",
			"task_id", task.TaskID,
			"type", task.Type,
			"repo", task.Repository,
			"branch", task.Branch,
		)

		prompt := fmt.Sprintf("Task: %s\nRepository: %s\nBranch: %s\n\n%s",
			task.Type, task.Repository, task.Branch, task.Description)
		if task.PRNumber > 0 {
			prompt = fmt.Sprintf("PR #%d\n%s", task.PRNumber, prompt)
		}

		for event, err := range r.Run(ctx, "system", evt.SessionID,
			genai.NewUserContent(genai.Text(prompt)), agent.RunConfig{}) {
			if err != nil {
				return fmt.Errorf("agent error: %w", err)
			}
			if event.IsFinalResponse() {
				log.Info("task complete", "task_id", task.TaskID, "session_id", evt.SessionID)
			}
		}
		return nil
	})
}
