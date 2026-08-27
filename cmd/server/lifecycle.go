package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"cave-sampling-permit/internal/httpapi"
)

func runBoundedSelfcheck(server *httpapi.Server, listener net.Listener, serveErrors <-chan error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	checkErr := executeSelfcheck(ctx, "http://"+listener.Addr().String())
	shutdownErr := server.Shutdown()
	serveErr := <-serveErrors
	if checkErr != nil {
		return fmt.Errorf("selfcheck failed: %w", checkErr)
	}
	if shutdownErr != nil {
		return fmt.Errorf("selfcheck shutdown failed: %w", shutdownErr)
	}
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return fmt.Errorf("selfcheck server failed: %w", serveErr)
	}
	fmt.Println("selfcheck ok: 已完成建案、违规检查、修订复检、独立复核、签发与核验")
	return nil
}
