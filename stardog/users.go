package stardog

import (
	"context"

	"go.uber.org/zap"
)

// UsersService handles communication with the user related
// methods of the Stardog API.
type UsersService service

type UsersList struct {
	Users []string `json:"users,omitempty"`
}

// Return list of existing users in database
func (s *UsersService) List(ctx context.Context) (*UsersList, *Response, error) {
	// Create new logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	u := "admin/users"
	req, err := s.client.NewRequest("GET", u, nil)

	if err != nil {
		logger.Error("Error creating new request", zap.Error(err))
		return nil, nil, err
	}

	users := new(UsersList)
	resp, err := s.client.Do(ctx, req, users)
	if err != nil {
		logger.Error("Error performing request", zap.Error(err))
		return users, resp, err
	}
	logger.Info("Successfully retrieved users list")
	return users, resp, nil
}
