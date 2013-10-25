package shredder

import (
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/storeadapter"
)

type Shredder struct {
	storeAdapter storeadapter.StoreAdapter
	logger       logger.Logger
}

func New(storeAdapter storeadapter.StoreAdapter, logger logger.Logger) *Shredder {
	return &Shredder{storeAdapter, logger}
}

func (s *Shredder) Shred() error {
	node, err := s.storeAdapter.ListRecursively("/")
	if err != nil {
		s.logger.Error("Failed to recursively fetch /", err)
		return err
	}

	s.shredNode(node)

	return nil
}

func (s *Shredder) shredNode(node storeadapter.StoreNode) bool {
	if node.Dir {
		if len(node.ChildNodes) == 0 {
			// ignoring errors -- best effort!
			s.logger.Info("Deleting Key", map[string]string{"Key": node.Key})
			s.storeAdapter.Delete(node.Key)
			return true
		} else {
			deletedAll := true

			for _, child := range node.ChildNodes {
				deletedAll = s.shredNode(child) && deletedAll
			}

			if deletedAll {
				// ignoring errors -- best effort!
				s.logger.Info("Deleting Key", map[string]string{"Key": node.Key})
				s.storeAdapter.Delete(node.Key)
				return true
			}
		}
	}

	return false
}
