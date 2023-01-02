package stardog

import (
	"context"
	"fmt"
	"net/http/httputil"
)

// UsersService handles communication with the user related
// methods of the Stardog API.
type UsersService service

type UsersList struct {
	Users []string `json:"users,omitempty"`
}

// Return list of existing users in database
func (s *UsersService) List(ctx context.Context) (*Response, error) {
	u := "admin/users"
	req, err := s.client.NewRequest("GET", u, nil)

	if err != nil {
		return nil, err
	}
	requestDump, err := httputil.DumpRequest(req, true)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(requestDump))

	var users UsersList
	resp, err := s.client.Do(ctx, req, &users)
	if err != nil {
		return resp, err
	}

	return resp, nil
}
