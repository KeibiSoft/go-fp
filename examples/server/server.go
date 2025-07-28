package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"

	mutable "github.com/KeibiSoft/go-fp/mutable"
)

func logErrorHandler(err error) error {
	if err != nil {
		log.Printf("[chain error] %v", err)
		// Optionally modify error, e.g., wrap or convert to nil if you want to suppress it
	}
	return err // return the original or modified error
}

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type UserStore struct {
	mu    sync.RWMutex
	users []User
}

func NewUserStore() *UserStore {
	return &UserStore{
		users: []User{
			{ID: 1, Name: "Alice", Age: 30},
			{ID: 2, Name: "Bob", Age: 22},
			{ID: 3, Name: "Carol", Age: 27},
		},
	}
}

func (s *UserStore) GetAll() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]User(nil), s.users...) // safe copy
}

func (s *UserStore) Add(u *User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u.ID = len(s.users) + 1
	s.users = append(s.users, *u)
}

// Lift DecodeJSON to Wrapper
func DecodeJSONWrapper[T any](r io.Reader) *mutable.Wrapper[T] {
	var val T
	err := json.NewDecoder(r).Decode(&val)
	w := mutable.Lift(&val, nil)
	if err != nil {
		return w.WithError(err)
	}
	return w
}

type NilStruct struct{}

// Lift EncodeJSON with side-effect into Wrapper
func EncodeJSONWrapper(wr http.ResponseWriter, v any) *mutable.Wrapper[NilStruct] {
	wr.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(wr).Encode(v)
	w := mutable.Lift(new(NilStruct), nil)
	if err != nil {
		return w.WithError(err)
	}
	return w
}

func (s *UserStore) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	users := s.GetAll()
	c1 := mutable.New(&users, logErrorHandler).
		FlatMap(func(u *[]User) mutable.Wrapper[[]User] {
			return *mutable.Lift(u, logErrorHandler)
		}).
		FlatMap(func(u *[]User) mutable.Wrapper[[]User] {
			// Actually wrap slice so we can call EncodeJSONWrapper with pointer
			return *mutable.Lift(u, logErrorHandler)
		})

	mutable.FlatMapU(c1, func(u *[]User) mutable.Wrapper[NilStruct] {
		return *EncodeJSONWrapper(w, u)
	}).
		Match(nil, func(err error) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
}

func (s *UserStore) handleAddUser(w http.ResponseWriter, r *http.Request) {
	DecodeJSONWrapper[User](r.Body).
		Then(func(u *User) (*User, error) {
			// Add user safely (mutate input user pointer)
			s.Add(u)
			return u, nil
		}).
		Then(func(_ *User) (*User, error) {
			w.WriteHeader(http.StatusCreated)
			return nil, nil
		}).
		Match(nil, func(err error) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		})
}

func main() {
	store := NewUserStore()

	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			store.handleGetUsers(w, r)
		case http.MethodPost:
			store.handleAddUser(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Println("Starting server at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
