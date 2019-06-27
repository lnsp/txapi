// Copyright 2019 espe
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/lnsp/txapi/pkg/api"
	"github.com/lnsp/txapi/pkg/data"
)

var (
	addr = flag.String("addr", ":8080", "addr to listen on")
	file = flag.String("file", "", "file to bind data to")
)

func main() {
	flag.Parse()

	done := make(chan os.Signal)
	signal.Notify(done, syscall.SIGINT)
	signal.Notify(done, syscall.SIGTERM)

	fileLogger := logrus.WithField("file", *file)
	datastore := data.NewInMemoryStore()
	if *file != "" {
		err := datastore.Load(*file)
		if err != nil {
			fileLogger.WithError(err).Error("failed to restore state from file")
		} else {
			fileLogger.Info("restored state from file")
		}
	}
	server := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      api.NewServer(datastore),
		Addr:         *addr,
	}

	// wait for graceful shutdown
	go func() {
		<-done
		logrus.Info("attempting graceful shutdown")
		if *file != "" {
			err := datastore.Save(*file)
			if err != nil {
				fileLogger.WithError(err).Error("failed to save state to file")
			} else {
				fileLogger.Info("saved state to file")
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	logrus.WithField("addr", *addr).Info("attempting to listen")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logrus.WithError(err).Fatal("failed to listen and serve")
	}
}
