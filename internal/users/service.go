package users

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(u *User) error {
	if err := s.db.Create(u).Error; err != nil {
		return fmt.Errorf("users: create: %w", err)
	}
	return nil
}

func (s *Service) FindByID(id string) (*User, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("users: invalid id: %w", err)
	}
	var u User
	if err := s.db.First(&u, "id = ?", uid).Error; err != nil {
		return nil, fmt.Errorf("users: find by id: %w", err)
	}
	return &u, nil
}

func (s *Service) FindByUsername(username string) (*User, error) {
	var u User
	if err := s.db.Where("username = ? OR email = ?", username, username).First(&u).Error; err != nil {
		return nil, fmt.Errorf("users: find by username: %w", err)
	}
	return &u, nil
}

func (s *Service) List() ([]User, error) {
	var list []User
	if err := s.db.Order("created_at ASC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("users: list: %w", err)
	}
	return list, nil
}

func (s *Service) Update(id string, req *UpdateUserRequest, hashFn func(string) (string, error)) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("users: invalid id: %w", err)
	}
	updates := map[string]any{"updated_at": time.Now()}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Role != "" && req.Role.Valid() {
		updates["role"] = req.Role
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.Password != "" {
		hash, err := hashFn(req.Password)
		if err != nil {
			return err
		}
		updates["password_hash"] = hash
	}
	return s.db.Model(&User{}).Where("id = ?", uid).Updates(updates).Error
}

func (s *Service) Delete(id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("users: invalid id: %w", err)
	}
	return s.db.Delete(&User{}, "id = ?", uid).Error
}

func (s *Service) UpdateLastLogin(id uuid.UUID) {
	now := time.Now()
	s.db.Model(&User{}).Where("id = ?", id).Update("last_login_at", now)
}

func (s *Service) CountSuperAdmins() (int64, error) {
	var count int64
	err := s.db.Model(&User{}).Where("role = ?", RoleSuperAdmin).Count(&count).Error
	return count, err
}
