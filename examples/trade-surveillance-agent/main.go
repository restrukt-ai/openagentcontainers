// trade-surveillance-agent screens every executed trade for regulatory
// violations: wash trading, front-running, spoofing, and position limit
// breaches. Session-isolated to handle exchange-scale event throughput.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/adk/tool/mcptoolset"
	"google.golang.org/genai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TradeExecuted is the event payload for trade-executed events
// (schemas/trade-executed.json).
type TradeExecuted struct {
	TradeID      string  `json:"trade_id"`
	TraderID     string  `json:"trader_id"`
	DeskID       string  `json:"desk_id,omitempty"`
	AccountID    string  `json:"account_id,omitempty"`
	SecurityID   string  `json:"security_id"`
	SecurityType string  `json:"security_type,omitempty"`
	Side         string  `json:"side"` // buy | sell | short-sell
	Quantity     float64 `json:"quantity"`
	Price        float64 `json:"price"`
	Notional     float64 `json:"notional,omitempty"`
	Venue        string  `json:"venue"`
	OrderID      string  `json:"order_id,omitempty"`
	ExecutedAt   string  `json:"executed_at"`
}

// --- Tools ---

type WashTradingArgs struct {
	TraderID   string  `json:"trader_id"   jsonschema:"description=Trader to check for wash trading"`
	SecurityID string  `json:"security_id" jsonschema:"description=Security being screened"`
	Side       string  `json:"side"        jsonschema:"description=Trade direction: buy or sell"`
	LookbackM  int     `json:"lookback_minutes" jsonschema:"description=Minutes of trade history to examine (default 60)"`
	Notional   float64 `json:"notional"    jsonschema:"description=Notional value of the current trade"`
}
type WashTradingResult struct {
	Flagged        bool     `json:"flagged"`
	MatchingTrades []string `json:"matching_trades"` // trade IDs of offsetting trades
	Explanation    string   `json:"explanation"`
}

// checkWashTrading detects buy/sell pairs in the same security within a short
// window — a common wash trading pattern. Market data MCP provides the history.
func checkWashTrading(_ tool.Context, args WashTradingArgs) (WashTradingResult, error) {
	lookback := args.LookbackM
	if lookback == 0 {
		lookback = 60
	}

	// Stub: in production, queries the market-data MCP for recent trades.
	// Real implementation checks for offsetting trades within the lookback window.
	oppositeSide := "sell"
	if args.Side == "sell" || args.Side == "short-sell" {
		oppositeSide = "buy"
	}
	_ = oppositeSide

	// Simulate no match for demo purposes.
	return WashTradingResult{
		Flagged:        false,
		MatchingTrades: nil,
		Explanation:    fmt.Sprintf("No offsetting %s trades found for %s in %s within %d minutes.", oppositeSide, args.SecurityID, args.TraderID, lookback),
	}, nil
}

type PositionLimitArgs struct {
	TraderID   string  `json:"trader_id"   jsonschema:"description=Trader whose positions to check"`
	SecurityID string  `json:"security_id" jsonschema:"description=Security to check position limits for"`
	Side       string  `json:"side"        jsonschema:"description=Trade direction"`
	Quantity   float64 `json:"quantity"    jsonschema:"description=Additional quantity from the new trade"`
}
type PositionLimitResult struct {
	Breached        bool    `json:"breached"`
	CurrentPosition float64 `json:"current_position"`
	Limit           float64 `json:"limit"`
	PostTradePos    float64 `json:"post_trade_position"`
	Explanation     string  `json:"explanation"`
}

// checkPositionLimits verifies the trader's post-trade position stays within
// regulatory and desk limits. Limits are fetched from the compliance rules MCP.
func checkPositionLimits(_ tool.Context, args PositionLimitArgs) (PositionLimitResult, error) {
	// Stub: simulates a position lookup and limit check.
	// Production implementation queries compliance-rules MCP for the limit
	// and market-data MCP for current position.
	const simulatedLimit = 1_000_000.0
	currentPos := 750_000.0 // simulated existing position

	delta := args.Quantity
	if args.Side == "sell" || args.Side == "short-sell" {
		delta = -args.Quantity
	}
	postTrade := currentPos + delta

	breached := postTrade > simulatedLimit || postTrade < -simulatedLimit
	explanation := fmt.Sprintf("Post-trade position %.0f vs limit ±%.0f.", postTrade, simulatedLimit)
	if breached {
		explanation = "BREACH: " + explanation
	}

	return PositionLimitResult{
		Breached:        breached,
		CurrentPosition: currentPos,
		Limit:           simulatedLimit,
		PostTradePos:    postTrade,
		Explanation:     explanation,
	}, nil
}

type FlagForReviewArgs struct {
	TradeID     string `json:"trade_id"     jsonschema:"description=Trade identifier to flag"`
	TraderID    string `json:"trader_id"    jsonschema:"description=Trader identifier"`
	ViolationType string `json:"violation_type" jsonschema:"description=Type of potential violation detected"`
	Severity    string `json:"severity"     jsonschema:"description=low | medium | high | critical"`
	Narrative   string `json:"narrative"    jsonschema:"description=Plain-language description of the concern"`
}

func flagForReview(_ tool.Context, args FlagForReviewArgs) (map[string]any, error) {
	// In production: calls the compliance-rules MCP to create a review alert.
	alertID := fmt.Sprintf("ALERT-%s-%d", args.TradeID, time.Now().UnixMilli())
	slog.Info("compliance alert created",
		"alert_id", alertID,
		"trade_id", args.TradeID,
		"trader_id", args.TraderID,
		"violation_type", args.ViolationType,
		"severity", args.Severity,
	)
	return map[string]any{
		"alert_id":       alertID,
		"violation_type": args.ViolationType,
		"severity":       args.Severity,
	}, nil
}

// --- Agent ---

func buildAgent(ctx context.Context) (agent.Agent, error) {
	// Orchestrator injects inference credentials via:
	//   org.openagentcontainers.inference.api_base.env="OPENAI_BASE_URL"
	//   org.openagentcontainers.inference.api_key.env="OPENAI_API_KEY"
	model, err := gemini.NewModel(ctx, "gemini-2.0-flash", &genai.ClientConfig{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})
	if err != nil {
		return nil, fmt.Errorf("creating model: %w", err)
	}

	washTradingTool, _ := functiontool.New(functiontool.Config{
		Name:        "check_wash_trading",
		Description: "Detect buy/sell pairs in the same security within a short time window for the same trader.",
	}, checkWashTrading)

	positionLimitTool, _ := functiontool.New(functiontool.Config{
		Name:        "check_position_limits",
		Description: "Verify the trader's post-trade position stays within regulatory and desk limits.",
	}, checkPositionLimits)

	flagReviewTool, _ := functiontool.New(functiontool.Config{
		Name:        "flag_for_review",
		Description: "Create a compliance alert for a suspicious trade. Use when a violation is detected.",
	}, flagForReview)

	// Market data MCP server — bearer token injected by orchestrator (spec §5.3).
	marketDataToolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.CommandTransport{
			Command: exec.Command("market-data-mcp-server", "--token", os.Getenv("MARKET_DATA_TOKEN")),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating market-data toolset: %w", err)
	}

	// Compliance rules MCP server — DCR credentials injected by orchestrator (spec §5.3).
	complianceToolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.CommandTransport{
			Command: exec.Command("compliance-mcp-server",
				"--client-id", os.Getenv("COMPLIANCE_CLIENT_ID"),
				"--client-secret", os.Getenv("COMPLIANCE_CLIENT_SECRET"),
			),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating compliance toolset: %w", err)
	}

	return llmagent.New(llmagent.Config{
		Name:  "trade-surveillance-agent",
		Model: model,
		Instruction: `You are a real-time trade surveillance analyst. For each executed trade,
run all applicable surveillance checks and flag violations immediately.

Checks to run for every trade:
1. check_wash_trading — look for offsetting trades in the same security within 60 minutes.
2. check_position_limits — verify the post-trade position stays within regulatory limits.
3. Use the market data tools to check for unusual price impact relative to recent VWAP.
4. Use the compliance tools to check if this trader has open orders on the opposite side
   (front-running indicator).

Flag using flag_for_review with appropriate severity:
- critical: confirmed wash trade, position limit breach, or front-running with clear evidence
- high: strong indicators requiring same-day review
- medium: patterns warranting end-of-day review

If no violations are detected, respond with a brief "CLEAR" summary listing the checks
performed and their results. Speed matters — surveillance alerts must be actionable
within seconds of execution.`,
		Tools:    []tool.Tool{washTradingTool, positionLimitTool, flagReviewTool},
		Toolsets: []tool.Toolset{marketDataToolset, complianceToolset},
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

	// Session-isolated: one process handles all concurrent trade events.
	// The orchestrator routes each trade to this process by session_id (spec §5.7).
	r, err := runner.New(runner.Config{
		AppName:           "trade-surveillance-agent",
		Agent:             ag,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		log.Error("creating runner", "error", err)
		os.Exit(1)
	}

	runHarness(ctx, func(ctx context.Context, evt OACEvent) error {
		var trade TradeExecuted
		if err := json.Unmarshal(evt.Payload, &trade); err != nil {
			return fmt.Errorf("decoding trade-executed event: %w", err)
		}

		log.Info("screening trade",
			"trade_id", trade.TradeID,
			"trader_id", trade.TraderID,
			"security", trade.SecurityID,
			"side", trade.Side,
			"notional", trade.Notional,
			"session_id", evt.SessionID,
		)

		prompt := fmt.Sprintf(
			"Trade ID: %s\nTrader: %s (desk: %s)\nSecurity: %s (%s)\n"+
				"Side: %s | Qty: %.0f | Price: %.4f | Notional: $%.2f\n"+
				"Venue: %s | Executed: %s",
			trade.TradeID, trade.TraderID, trade.DeskID,
			trade.SecurityID, trade.SecurityType,
			trade.Side, trade.Quantity, trade.Price, trade.Notional,
			trade.Venue, trade.ExecutedAt,
		)

		for event, err := range r.Run(ctx, "system", evt.SessionID,
			genai.NewUserContent(genai.Text(prompt)), agent.RunConfig{}) {
			if err != nil {
				return fmt.Errorf("agent error: %w", err)
			}
			if event.IsFinalResponse() {
				log.Info("screening complete", "trade_id", trade.TradeID, "session_id", evt.SessionID)
			}
		}
		return nil
	})
}
