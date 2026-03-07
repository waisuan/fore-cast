package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type StoreSuite struct {
	suite.Suite
}

func (s *StoreSuite) TestCreateGetDelete() {
	store := NewStore(24 * time.Hour)
	sid, err := store.Create("user1")
	s.Require().NoError(err)
	s.Require().NotEmpty(sid)

	data := store.Get(sid)
	s.Require().NotNil(data)
	s.Assert().Equal("user1", data.UserName)

	store.Delete(sid)
	data = store.Get(sid)
	s.Assert().Nil(data)
}

func (s *StoreSuite) TestExpiry() {
	store := NewStore(10 * time.Millisecond)
	sid, err := store.Create("u")
	s.Require().NoError(err)

	time.Sleep(15 * time.Millisecond)
	data := store.Get(sid)
	s.Assert().Nil(data)
}

func TestStoreSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(StoreSuite))
}
