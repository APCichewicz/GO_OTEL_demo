package repositories

import (
	"context"

	"github.com/apcichewicz/scratch/database"
	"github.com/jackc/pgx/v5/pgtype"
)

type AuthRepository interface {
	GetUserByOAuth(ctx context.Context, provider, oauthID string) (database.User, error)
	InsertOAuthUser(ctx context.Context, email, name, provider, oauthID string) (database.User, error)
}

type authRepository struct {
	db *database.Queries
}

func NewAuthRepository(db *database.Queries) AuthRepository {
	return &authRepository{db: db}
}

func (r *authRepository) GetUserByOAuth(ctx context.Context, provider, oauthID string) (database.User, error) {
	user, err := r.db.GetUserByOAuth(ctx, database.GetUserByOAuthParams{
		OauthProvider: pgtype.Text{String: provider, Valid: true},
		OauthID:       pgtype.Text{String: oauthID, Valid: true},
	})
	if err != nil {
		return database.User{}, err
	}
	return user, nil
}

func (r *authRepository) InsertOAuthUser(ctx context.Context, email, name, provider, oauthID string) (database.User, error) {
	user, err := r.db.InsertOAuthUser(ctx, database.InsertOAuthUserParams{
		Email:         email,
		Name:          name,
		OauthProvider: pgtype.Text{String: provider, Valid: true},
		OauthID:       pgtype.Text{String: oauthID, Valid: true},
	})
	if err != nil {
		return database.User{}, err
	}
	return user, nil
}
