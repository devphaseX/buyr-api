package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
)

type Review struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ProductID string    `json:"product_id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ReviewStore interface {
	Create(ctx context.Context, review *Review) error
	Delete(ctx context.Context, productID, reviewID string) error
	GetByProductID(ctx context.Context, productID string, filter PaginateQueryFilter) ([]*ReviewWithDetails, Metadata, error)
	GetReviewRatingAnalytics(ctx context.Context, productID string) (*ReviewRatingAnalytics, error)
}
type ReviewModel struct {
	db *sql.DB
}

func NewReviewModel(db *sql.DB) ReviewStore {
	return &ReviewModel{db}
}

func (m *ReviewModel) Create(ctx context.Context, review *Review) error {
	query := `INSERT INTO reviews(id, user_id, product_id, rating, comment) VALUES($1, $2, $3, $4, $5) RETURNING created_at, updated_at`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	review.ID = db.GenerateULID()

	args := []any{review.ID, review.UserID, review.ProductID, review.Rating, review.Comment}

	err := m.db.QueryRowContext(ctx, query, args...).Scan(&review.CreatedAt, &review.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

type ReviewWithDetails struct {
	Review
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

func (m *ReviewModel) GetByProductID(ctx context.Context, productID string, filter PaginateQueryFilter) ([]*ReviewWithDetails, Metadata, error) {
	query := fmt.Sprintf(`
			  SELECT count(r.id) over(), r.id, r.user_id, r.product_id, r.rating,
			  r.comment, r.created_at, r.updated_at, nu.first_name || ' ' || nu.last_name,
			  u.avatar_url
		      FROM reviews r
			  INNER JOIN users u ON u.id = r.user_id
			  INNER JOIN normal_users nu on nu.user_id = u.id
			  WHERE r.product_id = $1
			  ORDER BY r.%s %s
			  LIMIT $2 OFFSET $3
					`, filter.SortColumn(), filter.SortDirection())

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	rows, err := m.db.QueryContext(ctx, query, productID, filter.Limit(), filter.Offset())

	if err != nil {
		return nil, Metadata{}, fmt.Errorf("failed to query comments: %w", err)
	}

	defer rows.Close()

	var (
		reviews      = []*ReviewWithDetails{}
		totalRecords int
	)

	for rows.Next() {

		var (
			review    = &ReviewWithDetails{}
			avatarURL sql.NullString
		)

		err := rows.Scan(
			&totalRecords, &review.ID, &review.UserID, &review.ProductID,
			&review.Rating, &review.Comment, &review.CreatedAt, &review.UpdatedAt,
			&review.Username, &avatarURL,
		)

		if err != nil {
			return nil, Metadata{}, err
		}

		if avatarURL.Valid {
			review.AvatarURL = avatarURL.String
		}

		reviews = append(reviews, review)
	}

	if rows.Err() != nil {
		return nil, Metadata{}, fmt.Errorf("error after iterating over reviews rows: %w", err)

	}

	metadata := calculateMetadata(totalRecords, filter.Page, filter.PageSize)
	return reviews, metadata, nil
}

func (m *ReviewModel) Delete(ctx context.Context, productID, reviewID string) error {
	query := `
			DELETE FROM reviews
			WHERE id = $1 and product_id = $2
		`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	result, err := m.db.ExecContext(ctx, query, reviewID, productID)
	if err != nil {
		return err
	}

	// Check if the review was actually deleted
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

type ReviewRatingAnalytics struct {
	Ratings           map[int]int64 `json:"ratings"`
	TotalAverageRate  float64       `json:"total_average_rate"`
	TotalReviewsCount int64         `json:"total_reviews_count"`
}

func (m *ReviewModel) GetReviewRatingAnalytics(ctx context.Context, productID string) (*ReviewRatingAnalytics, error) {
	query := `
	select r.rating, count(rs.id)
		as count  from
		(
			select unnest(array[1, 2, 3, 4, 5]) as rating
		) as r
		LEFT JOIN reviews rs ON rs.rating = r.rating AND rs.product_id = $1
		GROUP BY r.rating
		ORDER BY r.rating DESC;

	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	rows, err := m.db.QueryContext(ctx, query, productID)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	analytics := &ReviewRatingAnalytics{
		Ratings: make(map[int]int64),
	}

	var (
		totalSum   int64
		totalCount int64
	)

	for rows.Next() {
		var (
			rating int
			count  int64
		)

		if err := rows.Scan(&rating, &count); err != nil {
			return nil, err
		}

		analytics.Ratings[rating] = count

		totalSum += int64(rating) * count
		totalCount += count
		analytics.TotalReviewsCount = totalCount

	}

	if totalCount > 0 {
		analytics.TotalAverageRate = float64(totalSum) / float64(totalCount)
	}

	return analytics, nil

}
