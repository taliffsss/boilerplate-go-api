package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go-api-boilerplate/database"
	"go-api-boilerplate/libraries"
	"go-api-boilerplate/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrInvalidID      = errors.New("invalid ID format")
)

// UserRepository defines user-specific repository methods
type UserRepository interface {
	libraries.Repository[models.User]

	// User-specific methods
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByRole(ctx context.Context, role string) ([]models.User, error)
	FindActive(ctx context.Context) ([]models.User, error)
	FindVerified(ctx context.Context) ([]models.User, error)
	Search(ctx context.Context, query string) ([]models.User, error)
	UpdateLastLogin(ctx context.Context, id any) error
	VerifyEmail(ctx context.Context, id any) error
	ChangePassword(ctx context.Context, id any, hashedPassword string) error
	CountByRole(ctx context.Context, role string) (int64, error)
}

// userRepository implements UserRepository
type userRepository struct {
	libraries.Repository[models.User]
	db         *database.DB
	collection *mongo.Collection // For MongoDB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *database.DB) UserRepository {
	var baseRepo libraries.Repository[models.User]
	var collection *mongo.Collection

	if database.IsMongoDB() {
		// MongoDB implementation
		collection = db.MongoDB.Collection("users")
		baseRepo = libraries.NewMongoRepository[models.User](collection, models.User{})
	} else {
		// SQL implementation (GORM)
		baseRepo = libraries.NewGormRepository[models.User](db, models.User{}, "users")
	}

	return &userRepository{
		Repository: baseRepo,
		db:         db,
		collection: collection,
	}
}

// FindByEmail finds a user by email
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	result, err := r.Where("email", email).First(ctx)
	if err != nil {
		if err == ErrRecordNotFound {
			return nil, fmt.Errorf("user with email %s not found", email)
		}
		return nil, err
	}
	return result, nil
}

// FindByRole finds all users with a specific role
func (r *userRepository) FindByRole(ctx context.Context, role string) ([]models.User, error) {
	return r.Where("role", role).Find(ctx)
}

// FindActive finds all active users
func (r *userRepository) FindActive(ctx context.Context) ([]models.User, error) {
	return r.Where("is_active", true).Find(ctx)
}

// FindVerified finds all verified users
func (r *userRepository) FindVerified(ctx context.Context) ([]models.User, error) {
	return r.Where("email_verified", true).Find(ctx)
}

// Search searches users by name or email
func (r *userRepository) Search(ctx context.Context, query string) ([]models.User, error) {
	if database.IsMongoDB() {
		// MongoDB text search
		filter := bson.M{
			"$or": []bson.M{
				{"name": bson.M{"$regex": query, "$options": "i"}},
				{"email": bson.M{"$regex": query, "$options": "i"}},
			},
		}

		cursor, err := r.collection.Find(ctx, filter)
		if err != nil {
			return nil, err
		}
		defer cursor.Close(ctx)

		var users []models.User
		if err := cursor.All(ctx, &users); err != nil {
			return nil, err
		}

		return users, nil
	} else {
		// SQL LIKE search
		searchPattern := "%" + query + "%"
		var users []models.User
		err := r.db.Read.WithContext(ctx).
			Where("name LIKE ? OR email LIKE ?", searchPattern, searchPattern).
			Find(&users).Error
		return users, err
	}
}

// UpdateLastLogin updates the last login timestamp
func (r *userRepository) UpdateLastLogin(ctx context.Context, id any) error {
	now := time.Now()
	return r.Update(ctx, id, &models.User{LastLoginAt: &now})
}

// VerifyEmail marks email as verified
func (r *userRepository) VerifyEmail(ctx context.Context, id any) error {
	now := time.Now()
	user := &models.User{
		EmailVerified:   true,
		EmailVerifiedAt: &now,
	}
	return r.Update(ctx, id, user)
}

// ChangePassword updates user password
func (r *userRepository) ChangePassword(ctx context.Context, id any, hashedPassword string) error {
	return r.Update(ctx, id, &models.User{Password: hashedPassword})
}

// CountByRole counts users by role
func (r *userRepository) CountByRole(ctx context.Context, role string) (int64, error) {
	return r.Where("role", role).Count(ctx)
}

// UserMongoRepository is a MongoDB-specific implementation for UserMongo model
type UserMongoRepository interface {
	libraries.Repository[models.UserMongo]

	// MongoDB-specific user methods
	FindByEmail(ctx context.Context, email string) (*models.UserMongo, error)
	FindByRole(ctx context.Context, role string) ([]models.UserMongo, error)
	FindActive(ctx context.Context) ([]models.UserMongo, error)
	Search(ctx context.Context, query string) ([]models.UserMongo, error)
	UpdateLastLogin(ctx context.Context, id any) error
}

// userMongoRepository implements UserMongoRepository
type userMongoRepository struct {
	libraries.Repository[models.UserMongo]
	collection *mongo.Collection
}

// NewUserMongoRepository creates a new MongoDB user repository
func NewUserMongoRepository(db *mongo.Database) UserMongoRepository {
	collection := db.Collection("users")
	baseRepo := libraries.NewMongoRepository[models.UserMongo](collection, models.UserMongo{})

	return &userMongoRepository{
		Repository: baseRepo,
		collection: collection,
	}
}

// FindByEmail finds a user by email in MongoDB
func (r *userMongoRepository) FindByEmail(ctx context.Context, email string) (*models.UserMongo, error) {
	result, err := r.Where("email", email).First(ctx)
	if err != nil {
		if err == ErrRecordNotFound {
			return nil, fmt.Errorf("user with email %s not found", email)
		}
		return nil, err
	}
	return result, nil
}

// FindByRole finds all users with a specific role in MongoDB
func (r *userMongoRepository) FindByRole(ctx context.Context, role string) ([]models.UserMongo, error) {
	return r.Where("role", role).Find(ctx)
}

// FindActive finds all active users in MongoDB
func (r *userMongoRepository) FindActive(ctx context.Context) ([]models.UserMongo, error) {
	return r.Where("is_active", true).Find(ctx)
}

// Search searches users by name or email in MongoDB
func (r *userMongoRepository) Search(ctx context.Context, query string) ([]models.UserMongo, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"name": bson.M{"$regex": query, "$options": "i"}},
			{"email": bson.M{"$regex": query, "$options": "i"}},
		},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []models.UserMongo
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}

	return users, nil
}

// UpdateLastLogin updates the last login timestamp in MongoDB
func (r *userMongoRepository) UpdateLastLogin(ctx context.Context, id any) error {
	objectID, err := r.toObjectID(id)
	if err != nil {
		return ErrInvalidID
	}

	update := bson.M{
		"$set": bson.M{
			"last_login_at": primitive.NewDateTimeFromTime(time.Now()),
			"updated_at":    primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": objectID}, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// Helper method to convert to ObjectID (reused from mongo_repository.go)
func (r *userMongoRepository) toObjectID(id any) (primitive.ObjectID, error) {
	switch v := id.(type) {
	case primitive.ObjectID:
		return v, nil
	case string:
		return primitive.ObjectIDFromHex(v)
	default:
		return primitive.NilObjectID, ErrInvalidID
	}
}
