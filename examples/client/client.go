package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	immutable "github.com/KeibiSoft/go-fp/immutable" // replace with your module path
)

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// Helper for our reader closer.
func ReadAllChain(r io.ReadCloser) immutable.Chain[[]byte] {
	data, err := io.ReadAll(r)
	if err != nil {
		return immutable.Wrap[[]byte](nil).WithError(err)
	}

	return immutable.Wrap(data)
}

// Helper for our json marshal.
func UnmarshalChain[T any](data []byte) immutable.Chain[T] {
	var val T
	if err := json.Unmarshal(data, &val); err != nil {
		return immutable.Wrap(val).WithError(err)
	}
	return immutable.Wrap(val)
}

func parseUsers(resp *http.Response) immutable.Chain[[]User] {
	// This can error, but the error can be lifted into our monadic chain.
	defer resp.Body.Close()
	bodyChain := immutable.Wrap(resp.Body)
	// Bind to ReadAllChain (lifted io.ReadAll)
	dataChain := immutable.Bind(bodyChain, ReadAllChain)
	// Bind to UnmarshalChain (lifted json.Unmarshal for []User)
	usersChain := immutable.Bind(dataChain, UnmarshalChain[[]User])
	return usersChain
}

func GetChain(url string) immutable.Chain[*http.Response] {
	return immutable.LiftResult(func() (*http.Response, error) {
		return http.Get(url)
	})
}

func fetchUsers() immutable.Chain[[]User] {
	// Start with lifting the URL string
	urlChain := immutable.Wrap("http://localhost:8080/users")
	// Bind urlChain to GetChain (lifted http.Get)
	respChain := immutable.Bind(urlChain, GetChain)
	// Bind respChain to parseUsers (parses http.Response to Chain[[]User])
	usersChain := immutable.Bind(respChain, parseUsers)
	return usersChain
}

func main() {
	usersChain := fetchUsers()

	// Filter users older than 25, map to their names, print
	usersChain.
		Then(func(users []User) ([]User, error) {
			var filtered []User
			for _, u := range users {
				if u.Age > 25 {
					filtered = append(filtered, u)
				}
			}
			return filtered, nil
		}).
		Map(func(users []User) []User {
			fmt.Println("Users older than 25:")
			for _, u := range users {
				fmt.Printf("  ID:%d Name:%s Age:%d\n", u.ID, u.Name, u.Age)
			}
			return users
		}).
		Match(nil, func(err error) {
			log.Println("Error fetching users:", err)
		})
}
