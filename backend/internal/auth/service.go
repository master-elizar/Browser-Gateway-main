package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserInactive       = errors.New("user inactive")
	ErrRegistrationClosed = errors.New("registration closed")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidRefresh     = errors.New("invalid refresh token")
	ErrUserNotFound       = errors.New("user not found")
	ErrCannotModifySelf   = errors.New("cannot deactivate or delete yourself")
	ErrLastSuperAdmin     = errors.New("cannot remove the last SUPER_ADMIN")
	ErrSetupRequired      = errors.New("initial setup required")
	ErrInvalidSetupKey    = errors.New("invalid setup key")
	ErrSetupComplete      = errors.New("setup already completed")
	ErrWrongPassword      = errors.New("current password is incorrect")
)

type Service struct {
	db     *gorm.DB
	tokens *TokenService
}

func NewService(db *gorm.DB, tokens *TokenService) *Service {
	return &Service{db: db, tokens: tokens}
}

type TokenPair struct {
	AccessToken  string       `json:"accessToken"`
	RefreshToken string       `json:"refreshToken"`
	User         domain.User  `json:"user"`
}

func publicUser(u domain.User) domain.User {
	u.PasswordHash = ""
	return u
}

func (s *Service) CountUsers() (int64, error) {
	var n int64
	err := s.db.Model(&domain.User{}).Count(&n).Error
	return n, err
}

func (s *Service) AllowRegistration() (bool, error) {
	var settings domain.AppSettings
	if err := s.db.First(&settings).Error; err != nil {
		return false, err
	}
	return settings.AllowRegistration, nil
}

func (s *Service) Register(email, password, displayName string) (*TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, fmt.Errorf("email required")
	}
	if err := s.ValidatePassword(password); err != nil {
		return nil, err
	}

	count, err := s.CountUsers()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, ErrSetupRequired
	}
	allow, err := s.AllowRegistration()
	if err != nil {
		return nil, err
	}
	if !allow {
		return nil, ErrRegistrationClosed
	}
	role := domain.RoleUser

	var existing domain.User
	err = s.db.Where("email = ?", email).First(&existing).Error
	if err == nil {
		return nil, ErrEmailTaken
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}

	user := domain.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: hash,
		DisplayName:  displayName,
		Role:         role,
		Active:       true,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}
	_ = s.writeAudit(user.ID, "auth.register", "user registered")
	return s.issuePair(&user)
}

func (s *Service) CompleteSetup(setupKey, email, password, displayName, keyFile string) (*TokenPair, error) {
	count, err := s.CountUsers()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSetupComplete
	}
	raw, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, ErrInvalidSetupKey
	}
	want := strings.TrimSpace(string(raw))
	if want == "" || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(setupKey)), []byte(want)) != 1 {
		return nil, ErrInvalidSetupKey
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, fmt.Errorf("email required")
	}
	if err := s.ValidatePassword(password); err != nil {
		return nil, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}
	user := domain.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: hash,
		DisplayName:  displayName,
		Role:         domain.RoleSuperAdmin,
		Active:       true,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}
	_ = os.Remove(keyFile)
	_ = s.writeAudit(user.ID, "auth.setup", "initial SUPER_ADMIN created via setup key")
	return s.issuePair(&user)
}

func (s *Service) Login(email, password string) (*TokenPair, error) {
	count, err := s.CountUsers()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, ErrSetupRequired
	}
	email = strings.ToLower(strings.TrimSpace(email))
	var user domain.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !user.Active {
		return nil, ErrUserInactive
	}
	if !VerifyPassword(user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	_ = s.writeAudit(user.ID, "auth.login", "user logged in")
	return s.issuePair(&user)
}

func (s *Service) Refresh(rawRefresh string) (*TokenPair, error) {
	hash := HashToken(rawRefresh)
	var rt domain.RefreshToken
	if err := s.db.Where("token_hash = ? AND revoked = false", hash).First(&rt).Error; err != nil {
		return nil, ErrInvalidRefresh
	}
	if time.Now().After(rt.ExpiresAt) {
		return nil, ErrInvalidRefresh
	}
	var user domain.User
	if err := s.db.First(&user, "id = ?", rt.UserID).Error; err != nil {
		return nil, ErrInvalidRefresh
	}
	if !user.Active {
		return nil, ErrUserInactive
	}
	// rotate
	_ = s.db.Model(&rt).Update("revoked", true).Error
	_ = s.writeAudit(user.ID, "auth.refresh", "refresh token rotated")
	return s.issuePair(&user)
}

func (s *Service) Logout(rawRefresh string) error {
	if rawRefresh == "" {
		return nil
	}
	hash := HashToken(rawRefresh)
	return s.db.Model(&domain.RefreshToken{}).
		Where("token_hash = ?", hash).
		Update("revoked", true).Error
}

func (s *Service) GetUser(id string) (*domain.User, error) {
	var user domain.User
	if err := s.db.First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	u := publicUser(user)
	return &u, nil
}

func (s *Service) ChangePassword(userID, currentPassword, newPassword string) error {
	var user domain.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		return ErrUserNotFound
	}
	if !VerifyPassword(user.PasswordHash, currentPassword) {
		return ErrWrongPassword
	}
	if err := s.ValidatePassword(newPassword); err != nil {
		return err
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.db.Model(&user).Update("password_hash", hash).Error; err != nil {
		return err
	}
	_ = s.WriteAudit(userID, "auth.password.change", "password changed by user")
	return nil
}

func (s *Service) BootstrapAdmin(email, password string) error {
	if email == "" || password == "" {
		return nil
	}
	count, err := s.CountUsers()
	if err != nil || count > 0 {
		return err
	}
	_, err = s.Register(email, password, "Admin")
	return err
}

func (s *Service) issuePair(user *domain.User) (*TokenPair, error) {
	access, err := s.tokens.IssueAccess(user)
	if err != nil {
		return nil, err
	}
	raw, err := RandomToken(32)
	if err != nil {
		return nil, err
	}
	rt := domain.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: HashToken(raw),
		ExpiresAt: time.Now().Add(s.tokens.RefreshTTL()),
	}
	if err := s.db.Create(&rt).Error; err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: raw,
		User:         publicUser(*user),
	}, nil
}

func (s *Service) writeAudit(userID, typ, msg string) error {
	return s.WriteAudit(userID, typ, msg)
}

func (s *Service) WriteAudit(userID, typ, msg string) error {
	ev := domain.AuditEvent{
		ID:        uuid.NewString(),
		Type:      typ,
		Message:   msg,
		CreatedAt: time.Now(),
	}
	if userID != "" {
		uid := userID
		ev.UserID = &uid
	}
	return s.db.Create(&ev).Error
}

func (s *Service) ListUsers() ([]domain.User, error) {
	var users []domain.User
	if err := s.db.Order("created_at asc").Find(&users).Error; err != nil {
		return nil, err
	}
	out := make([]domain.User, 0, len(users))
	for _, u := range users {
		out = append(out, publicUser(u))
	}
	return out, nil
}

func (s *Service) AdminCreateUser(email, password, displayName string, role domain.Role) (*domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := s.ValidatePassword(password); err != nil {
		return nil, err
	}
	if role != domain.RoleSuperAdmin && role != domain.RoleUser {
		role = domain.RoleUser
	}
	var existing domain.User
	err := s.db.Where("email = ?", email).First(&existing).Error
	if err == nil {
		return nil, ErrEmailTaken
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}
	user := domain.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: hash,
		DisplayName:  displayName,
		Role:         role,
		Active:       true,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}
	_ = s.WriteAudit(user.ID, "admin.user.create", fmt.Sprintf("created user %s", user.Email))
	u := publicUser(user)
	return &u, nil
}

func (s *Service) AdminPatchUser(id, actorID string, role *domain.Role, active *bool, password *string) (*domain.User, error) {
	var user domain.User
	if err := s.db.First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if active != nil && !*active && id == actorID {
		return nil, ErrCannotModifySelf
	}
	if role != nil && *role != domain.RoleSuperAdmin && user.Role == domain.RoleSuperAdmin {
		if err := s.ensureNotLastSuperAdmin(user.ID); err != nil {
			return nil, err
		}
	}
	if active != nil && !*active && user.Role == domain.RoleSuperAdmin {
		if err := s.ensureNotLastSuperAdmin(user.ID); err != nil {
			return nil, err
		}
	}

	updates := map[string]any{}
	if role != nil {
		if *role != domain.RoleSuperAdmin && *role != domain.RoleUser {
			return nil, fmt.Errorf("invalid role")
		}
		updates["role"] = *role
	}
	if active != nil {
		updates["active"] = *active
	}
	if password != nil {
		if err := s.ValidatePassword(*password); err != nil {
			return nil, err
		}
		hash, err := HashPassword(*password)
		if err != nil {
			return nil, err
		}
		updates["password_hash"] = hash
	}
	if len(updates) == 0 {
		u := publicUser(user)
		return &u, nil
	}
	if err := s.db.Model(&user).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.db.First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	_ = s.WriteAudit(actorID, "admin.user.update", fmt.Sprintf("updated user %s", user.Email))
	u := publicUser(user)
	return &u, nil
}

func (s *Service) AdminDeleteUser(id, actorID string) error {
	if id == actorID {
		return ErrCannotModifySelf
	}
	var user domain.User
	if err := s.db.First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	if user.Role == domain.RoleSuperAdmin {
		if err := s.ensureNotLastSuperAdmin(user.ID); err != nil {
			return err
		}
	}
	if err := s.db.Delete(&user).Error; err != nil {
		return err
	}
	_ = s.WriteAudit(actorID, "admin.user.delete", fmt.Sprintf("deleted user %s", user.Email))
	return nil
}

func (s *Service) ensureNotLastSuperAdmin(exceptID string) error {
	var n int64
	q := s.db.Model(&domain.User{}).Where("role = ? AND active = true", domain.RoleSuperAdmin)
	if exceptID != "" {
		q = q.Where("id <> ?", exceptID)
	}
	if err := q.Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return ErrLastSuperAdmin
	}
	return nil
}
