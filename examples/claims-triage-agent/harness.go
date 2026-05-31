// harness.go — OAC harness runtime (spec §4.4, §4.5)
//
// Manages the bidirectional ConnectRPC stream between this agent and the
// orchestrator. Receives OrchestratorEnvelope messages, dispatches events to
// the registered handler, and sends back HarnessEnvelope results.
//
// The ConnectRPC client and protobuf types are referenced here as they would
// appear once generated from the OAC schema. Replace the stub section with
// the generated client from the OAC Go SDK:
//
//	go get github.com/openagentcontainers/go-sdk
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

// OACEvent is an event delivered from the orchestrator via the harness stream.
type OACEvent struct {
	SessionID   string
	Channel     string
	Payload     []byte
	ContentType string
}

// OACEventHandler processes a single event. A non-nil error is reported as a
// failed EventResult on the orchestrator stream.
type OACEventHandler func(ctx context.Context, evt OACEvent) error

// runHarness connects to the orchestrator via mTLS and dispatches incoming
// events to handler. It blocks until ctx is cancelled or the stream closes.
func runHarness(ctx context.Context, handler OACEventHandler) {
	log := slog.Default()

	addr := mustEnv("ORCHESTRATOR_ADDR")

	// Load mTLS credentials written by the orchestrator at container startup (spec §5.5).
	cert, err := tls.LoadX509KeyPair("/run/secrets/harness.crt", "/run/secrets/harness.key")
	if err != nil {
		log.Error("loading mTLS cert/key", "error", err)
		os.Exit(1)
	}
	caBytes, err := os.ReadFile("/run/secrets/ca.crt")
	if err != nil {
		log.Error("loading CA cert", "error", err)
		os.Exit(1)
	}
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caBytes)

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				RootCAs:      caPool,
			},
		},
	}

	// Open a bidirectional ConnectRPC stream to the orchestrator (spec §4.4).
	//
	// Generated client usage (replace stub below once OAC Go SDK is available):
	//
	//   import (
	//       oacv1alpha2        "github.com/openagentcontainers/go-sdk/gen/openagentcontainers/v1alpha2"
	//       oacv1alpha2connect "github.com/openagentcontainers/go-sdk/gen/openagentcontainers/v1alpha2/v1alpha2connect"
	//   )
	//
	//   client := oacv1alpha2connect.NewOrchestratorClient(httpClient, "https://"+addr)
	//   stream := client.Connect(ctx)
	//
	//   for {
	//       msg, err := stream.Receive()
	//       if err != nil { return }
	//
	//       switch body := msg.Body.(type) {
	//       case *oacv1alpha2.OrchestratorEnvelope_Event:
	//           evt := OACEvent{
	//               SessionID:   msg.SessionId,
	//               Channel:     body.Event.Channel,
	//               Payload:     body.Event.Payload,
	//               ContentType: body.Event.ContentType,
	//           }
	//           handlerErr := handler(ctx, evt)
	//           stream.Send(&oacv1alpha2.HarnessEnvelope{
	//               SessionId: msg.SessionId,
	//               Body: &oacv1alpha2.HarnessEnvelope_Result{
	//                   Result: &oacv1alpha2.EventResult{
	//                       Success:      handlerErr == nil,
	//                       ErrorMessage: errString(handlerErr),
	//                   },
	//               },
	//           })
	//       case *oacv1alpha2.OrchestratorEnvelope_SessionEnd:
	//           log.Info("session ended", "session_id", msg.SessionId)
	//       }
	//   }

	// Stub: dial the orchestrator endpoint and confirm connectivity.
	// Replace with the generated ConnectRPC client above.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://%s/healthz", addr), nil)
	if err != nil {
		log.Error("building health check request", "error", err)
		os.Exit(1)
	}
	if _, err := httpClient.Do(req); err != nil {
		log.Error("orchestrator unreachable", "addr", addr, "error", err)
		os.Exit(1)
	}

	log.Info("harness connected to orchestrator", "addr", addr)
	<-ctx.Done()
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var not set", "key", key)
		os.Exit(1)
	}
	return v
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
