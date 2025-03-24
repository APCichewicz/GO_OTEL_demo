package repositories

import (
	"context"

	"github.com/apcichewicz/scratch/database"
)

type UserRepository interface {
	GetAllUsers(ctx context.Context) ([]database.User, error)
	InsertUser(ctx context.Context, user database.User) (database.User, error)
	GetUserByEmail(ctx context.Context, email string) (database.User, error)
	UpdateUser(ctx context.Context, user database.User) (database.User, error)
	DeleteUser(ctx context.Context, id int) (database.User, error)
}

type userRepository struct {
	db *database.Queries
}

func NewUserRepository(db *database.Queries) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetAllUsers(ctx context.Context) ([]database.User, error) {
	users, err := r.db.GetAllUsers(ctx)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *userRepository) InsertUser(ctx context.Context, user database.User) (database.User, error) {
	user, err := r.db.InsertUser(ctx, database.InsertUserParams{
		Email:    user.Email,
		Name:     user.Name,
		Password: user.Password,
	})
	if err != nil {
		return database.User{}, err
	}
	return user, nil
}

func (r *userRepository) GetUserByEmail(ctx context.Context, email string) (database.User, error) {
	user, err := r.db.GetUserByEmail(ctx, email)
	if err != nil {
		return database.User{}, err
	}
	return user, nil
}

func (r *userRepository) UpdateUser(ctx context.Context, user database.User) (database.User, error) {
	user, err := r.db.UpdateUser(ctx, database.UpdateUserParams{
		ID:       user.ID,
		Email:    user.Email,
		Name:     user.Name,
		Password: user.Password,
	})
	if err != nil {
		return database.User{}, err
	}
	return user, nil
}

func (r *userRepository) DeleteUser(ctx context.Context, id int) (database.User, error) {
	user, err := r.db.DeleteUser(ctx, int32(id))
	if err != nil {
		return database.User{}, err
	}
	return user, nil
}
