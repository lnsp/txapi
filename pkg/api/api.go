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

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/lnsp/txapi/pkg/data"
)

type Server struct {
	Model  data.Store
	router *mux.Router
}

func NewServer(model data.Store) *Server {
	server := &Server{
		Model: model,
	}
	server.setupRouter()
	return server
}

func (s *Server) setupRouter() {
	s.router = mux.NewRouter()
	s.router.HandleFunc("/user", s.createUser).Methods("POST")
	s.router.HandleFunc("/user/{uid}", s.deleteUser).Methods("DELETE")
	s.router.HandleFunc("/user/{uid}", s.showUser).Methods("GET")
	s.router.HandleFunc("/user/{uid}/tx", s.listTx).Methods("GET")
	s.router.HandleFunc("/user/{uid}/tx", s.insertTx).Methods("POST")
	s.router.HandleFunc("/user/{uid}/tx/{tid}", s.updateTx).Methods("PUT")
	s.router.HandleFunc("/user/{uid}/tx/{tid}", s.deleteTx).Methods("DELETE")
}

func (s *Server) error(w http.ResponseWriter, status int, msg string, err error) {
	logrus.WithError(err).WithField("statusCode", status).Error(msg)
	http.Error(w, fmt.Sprintf("%s: %v", msg, err), status)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	createRequest := struct {
		Name            string `json:"name,omitempty"`
		DefaultCurrency string `json:"currency,omitempty"`
	}{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&createRequest); err != nil {
		s.error(w, http.StatusBadRequest, "failed to decode user info", err)
		return
	}
	user, err := s.Model.Create(createRequest.Name)
	if err != nil {
		s.error(w, http.StatusInternalServerError, "failed to create user", err)
		return
	}
	logrus.WithField("user", user.ID).Info("created user")
	encoder := json.NewEncoder(w)
	createResponse := struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Currency string `json:"currency"`
	}{
		ID:       user.ID,
		Name:     user.Name,
		Currency: user.Currency,
	}
	if err := encoder.Encode(&createResponse); err != nil {
		s.error(w, http.StatusInternalServerError, "failed to encoder user", err)
		return
	}
}

func (s *Server) showUser(w http.ResponseWriter, r *http.Request) {
	store, err := s.user(r)
	if err != nil {
		s.error(w, http.StatusNotFound, "failed to get user store", err)
		return
	}
	userInfoResponse := struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Currency string `json:"currency"`
	}{
		ID:       store.Owner().ID,
		Name:     store.Owner().Name,
		Currency: store.Owner().Currency,
	}
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(&userInfoResponse); err != nil {
		s.error(w, http.StatusInternalServerError, "failed to encoder user", err)
		return
	}
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := mux.Vars(r)["uid"]
	if !ok {
		s.error(w, http.StatusBadRequest, "missing id param", nil)
		return
	}
	if err := s.Model.Delete(id); err != nil {
		s.error(w, http.StatusInternalServerError, "failed to delete user", err)
		return
	}
	logrus.WithField("user", id).Info("deleted user")
	w.Write([]byte("ok"))
}

func (s *Server) user(r *http.Request) (data.UserStore, error) {
	uid, ok := mux.Vars(r)["uid"]
	if !ok {
		return nil, errors.New("missing user param")
	}
	store, err := s.Model.Get(uid)
	if err != nil {
		return nil, fmt.Errorf("missing user %s", uid)
	}
	return store, nil
}

func (s *Server) insertTx(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	store, err := s.user(r)
	if err != nil {
		s.error(w, http.StatusNotFound, "failed to get user store", err)
		return
	}
	createRequest := struct {
		Name     string `json:"name,omitempty"`
		Amount   uint64 `json:"amount,omitempty"`
		Type     bool   `json:"type,omitempty"`
		Sender   string `json:"sender"`
		Currency string `json:"currency"`
	}{}
	if createRequest.Currency == "" {
		createRequest.Currency = store.Owner().Currency
		createRequest.Sender = store.Owner().Name
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&createRequest); err != nil {
		s.error(w, http.StatusBadRequest, "failed to decode request", err)
		return
	}
	tx, err := store.Create(&data.Transaction{
		Name:     createRequest.Name,
		Amount:   createRequest.Amount,
		Type:     createRequest.Type,
		Sender:   createRequest.Sender,
		Currency: createRequest.Currency,
	})
	if err != nil {
		s.error(w, http.StatusInternalServerError, "failed to store transaction", err)
		return
	}
	logrus.WithFields(logrus.Fields{
		"user":     store.Owner().ID,
		"tx":       tx.ID,
		"amount":   tx.Amount,
		"incoming": tx.Type,
	}).Info("created transaction")
	createResponse := struct {
		ID       string    `json:"id"`
		Name     string    `json:"name,omitempty"`
		Amount   uint64    `json:"amount,omitempty"`
		Type     bool      `json:"type,omitempty"`
		Sender   string    `json:"sender"`
		Currency string    `json:"currency"`
		Created  time.Time `json:"created"`
	}{
		ID:       tx.ID,
		Name:     tx.Name,
		Amount:   tx.Amount,
		Type:     tx.Type,
		Sender:   tx.Sender,
		Currency: tx.Currency,
		Created:  tx.Created,
	}
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(&createResponse); err != nil {
		s.error(w, http.StatusInternalServerError, "failed to encode response", err)
		return
	}
}

func (s *Server) listTx(w http.ResponseWriter, r *http.Request) {
	store, err := s.user(r)
	if err != nil {
		s.error(w, http.StatusNotFound, "failed to get user store", err)
		return
	}
	filters := make([]data.UserListFilter, 0)
	if sinceFilter := r.URL.Query().Get("since"); sinceFilter != "" {
		date, err := time.Parse(time.RFC3339, sinceFilter)
		if err != nil {
			s.error(w, http.StatusBadRequest, "invalid time filter", err)
			return
		}
		filters = append(filters, data.CreatedAfter(date))
	}
	txs, err := store.List(filters...)
	if err != nil {
		s.error(w, http.StatusInternalServerError, "failed to list transactions", err)
		return
	}
	listResponse := make([]struct {
		ID       string    `json:"id"`
		Name     string    `json:"name,omitempty"`
		Amount   uint64    `json:"amount,omitempty"`
		Type     bool      `json:"type,omitempty"`
		Sender   string    `json:"sender"`
		Currency string    `json:"currency"`
		Created  time.Time `json:"created"`
	}, len(txs))
	for i, tx := range txs {
		listResponse[i].ID = tx.ID
		listResponse[i].Name = tx.Name
		listResponse[i].Amount = tx.Amount
		listResponse[i].Type = tx.Type
		listResponse[i].Sender = tx.Sender
		listResponse[i].Currency = tx.Currency
		listResponse[i].Created = tx.Created
	}
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(listResponse); err != nil {
		s.error(w, http.StatusInternalServerError, "failed to encode response", err)
		return
	}
}

func (s *Server) updateTx(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	id, ok := mux.Vars(r)["tid"]
	if !ok {
		s.error(w, http.StatusBadRequest, "empty transaction id", nil)
		return
	}
	store, err := s.user(r)
	if err != nil {
		s.error(w, http.StatusNotFound, "failed to get user store", err)
		return
	}
	updateRequest := struct {
		Name     string `json:"name,omitempty"`
		Amount   uint64 `json:"amount,omitempty"`
		Type     bool   `json:"type,omitempty"`
		Sender   string `json:"sender"`
		Currency string `json:"currency"`
	}{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&updateRequest); err != nil {
		s.error(w, http.StatusBadRequest, "failed to decode update", err)
		return
	}
	tx, err := store.Update(&data.Transaction{
		ID:       id,
		Name:     updateRequest.Name,
		Amount:   updateRequest.Amount,
		Type:     updateRequest.Type,
		Sender:   updateRequest.Sender,
		Currency: updateRequest.Currency,
	})
	if err != nil {
		s.error(w, http.StatusInternalServerError, "failed to update transaction", err)
		return
	}
	logrus.WithFields(logrus.Fields{
		"user": store.Owner().ID,
		"tx":   tx.ID,
	}).Info("updated transaction")
	updateResponse := struct {
		ID       string    `json:"id"`
		Name     string    `json:"name,omitempty"`
		Amount   uint64    `json:"amount,omitempty"`
		Type     bool      `json:"type,omitempty"`
		Sender   string    `json:"sender"`
		Currency string    `json:"currency"`
		Created  time.Time `json:"created"`
	}{
		ID:       tx.ID,
		Name:     tx.Name,
		Amount:   tx.Amount,
		Type:     tx.Type,
		Sender:   tx.Sender,
		Currency: tx.Currency,
		Created:  tx.Created,
	}
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(&updateResponse); err != nil {
		s.error(w, http.StatusInternalServerError, "failed to encode transaction", err)
		return
	}
}

func (s *Server) deleteTx(w http.ResponseWriter, r *http.Request) {
	id, ok := mux.Vars(r)["tid"]
	if !ok {
		s.error(w, http.StatusBadRequest, "empty transaction id", nil)
		return
	}
	store, err := s.user(r)
	if err != nil {
		s.error(w, http.StatusNotFound, "failed to get user store", err)
		return
	}
	if err := store.Delete(id); err != nil {
		s.error(w, http.StatusInternalServerError, "failed to delete transaction", err)
		return
	}
	logrus.WithFields(logrus.Fields{
		"user": store.Owner().ID,
		"tx":   id,
	}).Info("deleted transaction")
	w.Write([]byte("ok"))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logrus.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
		"addr":   r.RemoteAddr,
	}).Debug("incoming request")
	s.router.ServeHTTP(w, r)
}
