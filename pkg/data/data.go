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

package data

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

var UserNotFound = errors.New("user not found")
var TransactionNotFound = errors.New("transaction not found")

type Transaction struct {
	ID       string
	Name     string
	Sender   string
	Amount   uint64
	Type     bool
	Currency string
	Created  time.Time
}

type User struct {
	ID       string
	Name     string
	Currency string
}

type Store interface {
	Create(name string) (*User, error)
	Get(id string) (UserStore, error)
	Delete(id string) error
}

type UserListFilter func(t *Transaction) bool

func CreatedAfter(t time.Time) UserListFilter {
	return func(tx *Transaction) bool {
		return tx.Created.After(t)
	}
}

type UserStore interface {
	Owner() *User
	List(filters ...UserListFilter) ([]*Transaction, error)
	Create(tx *Transaction) (*Transaction, error)
	Update(tx *Transaction) (*Transaction, error)
	Delete(id string) error
}

type InMemoryStore struct {
	usersLock sync.RWMutex
	users     map[string]*InMemoryUserStore
}

type InMemoryUserStore struct {
	user            *User
	transactionLock sync.RWMutex
	mapTransactions map[string]*Transaction
	transactions    []*Transaction
}

func (store *InMemoryStore) Create(name string) (*User, error) {
	user := &User{
		ID:       uuid.New().String(),
		Name:     name,
		Currency: "EUR",
	}
	store.usersLock.Lock()
	defer store.usersLock.Unlock()
	store.users[user.ID] = &InMemoryUserStore{
		user:            user,
		mapTransactions: make(map[string]*Transaction),
		transactions:    make([]*Transaction, 0),
	}
	return user, nil
}

func (store *InMemoryStore) Get(id string) (UserStore, error) {
	store.usersLock.RLock()
	defer store.usersLock.RUnlock()
	user, ok := store.users[id]
	if !ok {
		return nil, UserNotFound
	}
	return user, nil
}

func (store *InMemoryStore) Delete(id string) error {
	store.usersLock.Lock()
	defer store.usersLock.Unlock()
	_, ok := store.users[id]
	if !ok {
		return UserNotFound
	}
	delete(store.users, id)
	return nil
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		users: make(map[string]*InMemoryUserStore),
	}
}

func (store *InMemoryStore) Save(file string) error {
	store.usersLock.Lock()
	defer store.usersLock.Unlock()

	handle, err := os.Create(file)
	if err != nil {
		return err
	}
	defer handle.Close()
	encoder := json.NewEncoder(handle)
	users := make([]struct {
		User         *User
		Transactions []*Transaction
	}, len(store.users))
	i := 0
	for _, userStore := range store.users {
		users[i].User = userStore.user
		users[i].Transactions = userStore.transactions
		i++
	}
	if err := encoder.Encode(&users); err != nil {
		return err
	}
	return nil
}

func (store *InMemoryStore) Load(file string) error {
	store.usersLock.Lock()
	defer store.usersLock.Unlock()

	handle, err := os.Open(file)
	if err != nil {
		return err
	}
	defer handle.Close()

	store.users = make(map[string]*InMemoryUserStore)
	users := []struct {
		User         *User
		Transactions []*Transaction
	}{}
	decoder := json.NewDecoder(handle)
	if err := decoder.Decode(&users); err != nil {
		return err
	}
	for _, userStore := range users {
		memStore := &InMemoryUserStore{
			user:            userStore.User,
			transactions:    userStore.Transactions,
			mapTransactions: make(map[string]*Transaction),
		}
		for _, tx := range memStore.transactions {
			memStore.mapTransactions[tx.ID] = tx
		}
		store.users[userStore.User.ID] = memStore
	}
	return nil
}

func (store *InMemoryUserStore) List(filters ...UserListFilter) ([]*Transaction, error) {
	store.transactionLock.RLock()
	defer store.transactionLock.RUnlock()
	size := len(store.transactions) / 2
	output := make([]*Transaction, size)
	i := 0
	for _, tx := range store.transactions {
		ok := true
		for j := 0; j < len(filters) && ok; j++ {
			if !filters[j](tx) {
				ok = false
			}
		}
		if ok {
			if i < size {
				output[i] = tx
			} else {
				output = append(output, tx)
			}
		}
	}
	return output, nil
}

func (store *InMemoryUserStore) Create(tx *Transaction) (*Transaction, error) {
	tx.ID = uuid.New().String()
	tx.Created = time.Now()
	store.transactionLock.Lock()
	defer store.transactionLock.Unlock()
	store.transactions = append(store.transactions, tx)
	store.mapTransactions[tx.ID] = tx
	return tx, nil
}

func (store *InMemoryUserStore) Delete(id string) error {
	store.transactionLock.Lock()
	defer store.transactionLock.Unlock()
	size := len(store.transactions)
	tx, ok := store.mapTransactions[id]
	if !ok {
		return errors.New("transaction not found")
	}
	delete(store.mapTransactions, id)
	i := sort.Search(size, func(i int) bool {
		return tx.Created.Before(store.transactions[i].Created)
	})
	store.transactions[i], store.transactions[size-1] = store.transactions[size-1], store.transactions[i]
	store.transactions = store.transactions[:size-1]
	return nil
}

func (store *InMemoryUserStore) Update(tx *Transaction) (*Transaction, error) {
	store.transactionLock.RLock()
	store.transactionLock.RUnlock()
	val, ok := store.mapTransactions[tx.ID]
	if !ok {
		return nil, TransactionNotFound
	}
	val.Name = tx.Name
	val.Sender = tx.Sender
	val.Amount = tx.Amount
	val.Type = tx.Type
	val.Currency = tx.Currency
	return val, nil
}

func (store *InMemoryUserStore) Owner() *User {
	return store.user
}
