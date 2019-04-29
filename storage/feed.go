// Copyright 2017 Frédéric Guillot. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package storage // import "miniflux.app/storage"

import (
	"database/sql"
	"errors"
	"fmt"

	"miniflux.app/model"
	"miniflux.app/timezone"
)

// FeedExists checks if the given feed exists.
func (s *Storage) FeedExists(userID, feedID int64) bool {
	var result int
	query := `SELECT count(*) as c FROM feeds WHERE user_id=$1 AND id=$2`
	s.db.QueryRow(query, userID, feedID).Scan(&result)
	return result >= 1
}

// FeedURLExists checks if feed URL already exists.
func (s *Storage) FeedURLExists(userID int64, feedURL string) bool {
	var result int
	query := `SELECT count(*) as c FROM feeds WHERE user_id=$1 AND feed_url=$2`
	s.db.QueryRow(query, userID, feedURL).Scan(&result)
	return result >= 1
}

// CountFeeds returns the number of feeds that belongs to the given user.
func (s *Storage) CountFeeds(userID int64) int {
	var result int
	err := s.db.QueryRow(`SELECT count(*) FROM feeds WHERE user_id=$1`, userID).Scan(&result)
	if err != nil {
		return 0
	}

	return result
}

// CountErrorFeeds returns the number of feeds with parse errors that belong to the given user.
func (s *Storage) CountErrorFeeds(userID int64) int {
	var result int
	err := s.db.QueryRow(`SELECT count(*) FROM feeds WHERE user_id=$1 AND parsing_error_count>=$2`, userID, maxParsingError).Scan(&result)
	if err != nil {
		return 0
	}

	return result
}

// Feeds returns all feeds of the given user.
func (s *Storage) Feeds(userID int64) (model.Feeds, error) {
	feeds := make(model.Feeds, 0)
	query := `SELECT
		f.id, f.feed_url, f.site_url, f.title, f.etag_header, f.last_modified_header,
		f.user_id, f.checked_at at time zone u.timezone,
		f.parsing_error_count, f.parsing_error_msg,
		f.scraper_rules, f.rewrite_rules, f.crawler, f.user_agent,
		f.username, f.password,
		f.category_id, c.title as category_title,
		fi.icon_id,
		u.timezone
		FROM feeds f
		LEFT JOIN categories c ON c.id=f.category_id
		LEFT JOIN feed_icons fi ON fi.feed_id=f.id
		LEFT JOIN users u ON u.id=f.user_id
		WHERE f.user_id=$1
		ORDER BY f.parsing_error_count DESC, lower(f.title) ASC`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch feeds: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var feed model.Feed
		var iconID interface{}
		var tz string
		feed.Category = &model.Category{UserID: userID}

		err := rows.Scan(
			&feed.ID,
			&feed.FeedURL,
			&feed.SiteURL,
			&feed.Title,
			&feed.EtagHeader,
			&feed.LastModifiedHeader,
			&feed.UserID,
			&feed.CheckedAt,
			&feed.ParsingErrorCount,
			&feed.ParsingErrorMsg,
			&feed.ScraperRules,
			&feed.RewriteRules,
			&feed.Crawler,
			&feed.UserAgent,
			&feed.Username,
			&feed.Password,
			&feed.Category.ID,
			&feed.Category.Title,
			&iconID,
			&tz,
		)

		if err != nil {
			return nil, fmt.Errorf("unable to fetch feeds row: %v", err)
		}

		if iconID != nil {
			feed.Icon = &model.FeedIcon{FeedID: feed.ID, IconID: iconID.(int64)}
		}

		feed.CheckedAt = timezone.Convert(tz, feed.CheckedAt)
		feeds = append(feeds, &feed)
	}

	return feeds, nil
}

// FeedByID returns a feed by the ID.
func (s *Storage) FeedByID(userID, feedID int64) (*model.Feed, error) {
	var feed model.Feed
	var iconID interface{}
	var tz string
	feed.Category = &model.Category{UserID: userID}

	query := `
		SELECT
		f.id, f.feed_url, f.site_url, f.title, f.etag_header, f.last_modified_header,
		f.user_id, f.checked_at at time zone u.timezone,
		f.parsing_error_count, f.parsing_error_msg,
		f.scraper_rules, f.rewrite_rules, f.crawler, f.user_agent,
		f.username, f.password,
		f.category_id, c.title as category_title,
		fi.icon_id,
		u.timezone
		FROM feeds f
		LEFT JOIN categories c ON c.id=f.category_id
		LEFT JOIN feed_icons fi ON fi.feed_id=f.id
		LEFT JOIN users u ON u.id=f.user_id
		WHERE f.user_id=$1 AND f.id=$2`

	err := s.db.QueryRow(query, userID, feedID).Scan(
		&feed.ID,
		&feed.FeedURL,
		&feed.SiteURL,
		&feed.Title,
		&feed.EtagHeader,
		&feed.LastModifiedHeader,
		&feed.UserID,
		&feed.CheckedAt,
		&feed.ParsingErrorCount,
		&feed.ParsingErrorMsg,
		&feed.ScraperRules,
		&feed.RewriteRules,
		&feed.Crawler,
		&feed.UserAgent,
		&feed.Username,
		&feed.Password,
		&feed.Category.ID,
		&feed.Category.Title,
		&iconID,
		&tz,
	)

	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("unable to fetch feed #%d: %v", feedID, err)
	}

	if iconID != nil {
		feed.Icon = &model.FeedIcon{FeedID: feed.ID, IconID: iconID.(int64)}
	}

	feed.CheckedAt = timezone.Convert(tz, feed.CheckedAt)
	return &feed, nil
}

// CreateFeed creates a new feed.
func (s *Storage) CreateFeed(feed *model.Feed) error {
	sql := `
		INSERT INTO feeds
		(feed_url, site_url, title, category_id, user_id, etag_header, last_modified_header, crawler, user_agent, username, password)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`

	err := s.db.QueryRow(
		sql,
		feed.FeedURL,
		feed.SiteURL,
		feed.Title,
		feed.Category.ID,
		feed.UserID,
		feed.EtagHeader,
		feed.LastModifiedHeader,
		feed.Crawler,
		feed.UserAgent,
		feed.Username,
		feed.Password,
	).Scan(&feed.ID)
	if err != nil {
		return fmt.Errorf("unable to create feed %q: %v", feed.FeedURL, err)
	}

	for i := 0; i < len(feed.Entries); i++ {
		feed.Entries[i].FeedID = feed.ID
		feed.Entries[i].UserID = feed.UserID
		err := s.createEntry(feed.Entries[i])
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateFeed updates an existing feed.
func (s *Storage) UpdateFeed(feed *model.Feed) (err error) {
	query := `UPDATE feeds SET
		feed_url=$1, site_url=$2, title=$3, category_id=$4, etag_header=$5, last_modified_header=$6, checked_at=$7,
		parsing_error_msg=$8, parsing_error_count=$9, scraper_rules=$10, rewrite_rules=$11, crawler=$12, user_agent=$13,
		username=$14, password=$15
		WHERE id=$16 AND user_id=$17`

	_, err = s.db.Exec(query,
		feed.FeedURL,
		feed.SiteURL,
		feed.Title,
		feed.Category.ID,
		feed.EtagHeader,
		feed.LastModifiedHeader,
		feed.CheckedAt,
		feed.ParsingErrorMsg,
		feed.ParsingErrorCount,
		feed.ScraperRules,
		feed.RewriteRules,
		feed.Crawler,
		feed.UserAgent,
		feed.Username,
		feed.Password,
		feed.ID,
		feed.UserID,
	)

	if err != nil {
		return fmt.Errorf("unable to update feed #%d (%s): %v", feed.ID, feed.FeedURL, err)
	}

	return nil
}

// UpdateFeedError updates feed errors.
func (s *Storage) UpdateFeedError(feed *model.Feed) (err error) {
	query := `
		UPDATE feeds
		SET
			parsing_error_msg=$1,
			parsing_error_count=$2,
			checked_at=$3
		WHERE id=$4 AND user_id=$5`

	_, err = s.db.Exec(query,
		feed.ParsingErrorMsg,
		feed.ParsingErrorCount,
		feed.CheckedAt,
		feed.ID,
		feed.UserID,
	)

	if err != nil {
		return fmt.Errorf("unable to update feed error #%d (%s): %v", feed.ID, feed.FeedURL, err)
	}

	return nil
}

// RemoveFeed removes a feed.
func (s *Storage) RemoveFeed(userID, feedID int64) error {
	result, err := s.db.Exec("DELETE FROM feeds WHERE id = $1 AND user_id = $2", feedID, userID)
	if err != nil {
		return fmt.Errorf("unable to remove feed #%d: %v", feedID, err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("unable to remove feed #%d: %v", feedID, err)
	}

	if count == 0 {
		return errors.New("no feed has been removed")
	}

	return nil
}

// ResetFeedErrors removes all feed errors.
func (s *Storage) ResetFeedErrors() error {
	_, err := s.db.Exec(`UPDATE feeds SET parsing_error_count=0, parsing_error_msg=''`)
	return err
}
