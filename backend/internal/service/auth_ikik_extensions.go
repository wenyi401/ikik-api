package service

import (
	"context"

	"fmt"
)

func (s *AuthService) SetUserPrivateGroupProvisioner(provisioner UserPrivateGroupProvisioner) {
	if s == nil {
		return
	}
	s.privateGroupProvisioner = provisioner
}

func (s *AuthService) provisionUserPrivateGroups(ctx context.Context, userID int64) error {
	if s == nil || s.privateGroupProvisioner == nil || userID <= 0 {
		return nil
	}
	if err := s.privateGroupProvisioner.ProvisionUserPrivateGroups(ctx, userID); err != nil {
		return fmt.Errorf("provision user private groups: %w", err)
	}
	return nil
}
