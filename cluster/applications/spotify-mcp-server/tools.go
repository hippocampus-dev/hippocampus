package main

import (
	"context"
	"encoding/json"
	"fmt"

	"spotify-mcp-server/internal/spotify"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/xerrors"
)

const spotifyAPIBaseURL = "https://api.spotify.com/v1"

type contextTokenSource struct{}

func (t *contextTokenSource) OAuth20(ctx context.Context, _ spotify.OperationName) (spotify.OAuth20, error) {
	token, ok := ctx.Value(spotifyTokenKey).(string)
	if !ok || token == "" {
		return spotify.OAuth20{}, fmt.Errorf("no spotify token in context")
	}
	return spotify.OAuth20{Token: token}, nil
}

func tools(s *server.MCPServer, c *spotify.Client) {
	s.AddTool(
		mcp.NewTool("search_tracks",
			mcp.WithDescription("Search for tracks on Spotify"),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of results (1-50, default 20)")),
			mcp.WithNumber("offset", mcp.Description("Offset for pagination (default 0)")),
			mcp.WithString("market", mcp.Description("ISO 3166-1 alpha-2 country code")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, err := request.RequireString("query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			params := spotify.SearchParams{
				Q:    query,
				Type: []spotify.SearchTypeItem{spotify.SearchTypeItemTrack},
			}
			if v := request.GetInt("limit", 0); v > 0 {
				params.Limit = spotify.NewOptInt(v)
			}
			if v := request.GetInt("offset", 0); v > 0 {
				params.Offset = spotify.NewOptInt(v)
			}
			if v := request.GetString("market", ""); v != "" {
				params.Market = spotify.NewOptString(v)
			}

			response, err := c.Search(ctx, params)
			if err != nil {
				return nil, xerrors.Errorf("search failed: %w", err)
			}

			data, err := json.Marshal(response)
			if err != nil {
				return nil, xerrors.Errorf("failed to marshal response: %w", err)
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("get_track",
			mcp.WithDescription("Get Spotify catalog information for a single track identified by its Spotify ID"),
			mcp.WithString("id", mcp.Required(), mcp.Description("Spotify track ID")),
			mcp.WithString("market", mcp.Description("ISO 3166-1 alpha-2 country code")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := request.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			params := spotify.GetTrackParams{ID: id}
			if v := request.GetString("market", ""); v != "" {
				params.Market = spotify.NewOptString(v)
			}

			response, err := c.GetTrack(ctx, params)
			if err != nil {
				return nil, xerrors.Errorf("get track failed: %w", err)
			}

			data, err := json.Marshal(response)
			if err != nil {
				return nil, xerrors.Errorf("failed to marshal response: %w", err)
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("get_several_tracks",
			mcp.WithDescription("Get Spotify catalog information for multiple tracks based on their Spotify IDs"),
			mcp.WithString("ids", mcp.Required(), mcp.Description("Comma-separated list of Spotify track IDs (max 50)")),
			mcp.WithString("market", mcp.Description("ISO 3166-1 alpha-2 country code")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ids, err := request.RequireString("ids")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			params := spotify.GetSeveralTracksParams{Ids: ids}
			if v := request.GetString("market", ""); v != "" {
				params.Market = spotify.NewOptString(v)
			}

			response, err := c.GetSeveralTracks(ctx, params)
			if err != nil {
				return nil, xerrors.Errorf("get several tracks failed: %w", err)
			}

			data, err := json.Marshal(response)
			if err != nil {
				return nil, xerrors.Errorf("failed to marshal response: %w", err)
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("get_saved_tracks",
			mcp.WithDescription("Get a list of the songs saved in the current user's library. Requires user-library-read scope."),
			mcp.WithNumber("limit", mcp.Description("Maximum number of results (1-50, default 20)")),
			mcp.WithNumber("offset", mcp.Description("Offset for pagination (default 0)")),
			mcp.WithString("market", mcp.Description("ISO 3166-1 alpha-2 country code")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			params := spotify.GetUsersSavedTracksParams{}
			if v := request.GetInt("limit", 0); v > 0 {
				params.Limit = spotify.NewOptInt(v)
			}
			if v := request.GetInt("offset", 0); v > 0 {
				params.Offset = spotify.NewOptInt(v)
			}
			if v := request.GetString("market", ""); v != "" {
				params.Market = spotify.NewOptString(v)
			}

			response, err := c.GetUsersSavedTracks(ctx, params)
			if err != nil {
				return nil, xerrors.Errorf("get saved tracks failed: %w", err)
			}

			data, err := json.Marshal(response)
			if err != nil {
				return nil, xerrors.Errorf("failed to marshal response: %w", err)
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("check_saved_tracks",
			mcp.WithDescription("Check if one or more tracks are saved in the current user's library. Requires user-library-read scope."),
			mcp.WithString("ids", mcp.Required(), mcp.Description("Comma-separated list of Spotify track IDs (max 50)")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ids, err := request.RequireString("ids")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			response, err := c.CheckUsersSavedTracks(ctx, spotify.CheckUsersSavedTracksParams{Ids: ids})
			if err != nil {
				return nil, xerrors.Errorf("check saved tracks failed: %w", err)
			}

			data, err := json.Marshal(response)
			if err != nil {
				return nil, xerrors.Errorf("failed to marshal response: %w", err)
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("get_album_tracks",
			mcp.WithDescription("Get Spotify catalog information about an album's tracks"),
			mcp.WithString("id", mcp.Required(), mcp.Description("Spotify album ID")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of results (1-50, default 20)")),
			mcp.WithNumber("offset", mcp.Description("Offset for pagination (default 0)")),
			mcp.WithString("market", mcp.Description("ISO 3166-1 alpha-2 country code")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := request.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			params := spotify.GetAnAlbumsTracksParams{ID: id}
			if v := request.GetInt("limit", 0); v > 0 {
				params.Limit = spotify.NewOptInt(v)
			}
			if v := request.GetInt("offset", 0); v > 0 {
				params.Offset = spotify.NewOptInt(v)
			}
			if v := request.GetString("market", ""); v != "" {
				params.Market = spotify.NewOptString(v)
			}

			response, err := c.GetAnAlbumsTracks(ctx, params)
			if err != nil {
				return nil, xerrors.Errorf("get album tracks failed: %w", err)
			}

			data, err := json.Marshal(response)
			if err != nil {
				return nil, xerrors.Errorf("failed to marshal response: %w", err)
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("get_user_top_tracks",
			mcp.WithDescription("Get the current user's top tracks. Requires user-top-read scope."),
			mcp.WithString("time_range", mcp.Description("Time range: short_term (4 weeks), medium_term (6 months), long_term (years). Default: medium_term")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of results (1-50, default 20)")),
			mcp.WithNumber("offset", mcp.Description("Offset for pagination (default 0)")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			params := spotify.GetUsersTopArtistsAndTracksParams{Type: spotify.GetUsersTopArtistsAndTracksTypeTracks}
			if v := request.GetString("time_range", ""); v != "" {
				params.TimeRange = spotify.NewOptString(v)
			}
			if v := request.GetInt("limit", 0); v > 0 {
				params.Limit = spotify.NewOptInt(v)
			}
			if v := request.GetInt("offset", 0); v > 0 {
				params.Offset = spotify.NewOptInt(v)
			}

			response, err := c.GetUsersTopArtistsAndTracks(ctx, params)
			if err != nil {
				return nil, xerrors.Errorf("get user top tracks failed: %w", err)
			}

			data, err := json.Marshal(response)
			if err != nil {
				return nil, xerrors.Errorf("failed to marshal response: %w", err)
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("get_playlist_tracks",
			mcp.WithDescription("Get full details of the tracks in a playlist"),
			mcp.WithString("id", mcp.Required(), mcp.Description("Spotify playlist ID")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of results (1-100, default 100)")),
			mcp.WithNumber("offset", mcp.Description("Offset for pagination (default 0)")),
			mcp.WithString("market", mcp.Description("ISO 3166-1 alpha-2 country code")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := request.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			params := spotify.GetPlaylistsTracksParams{PlaylistID: id}
			if v := request.GetInt("limit", 0); v > 0 {
				params.Limit = spotify.NewOptInt(v)
			}
			if v := request.GetInt("offset", 0); v > 0 {
				params.Offset = spotify.NewOptInt(v)
			}
			if v := request.GetString("market", ""); v != "" {
				params.Market = spotify.NewOptString(v)
			}

			response, err := c.GetPlaylistsTracks(ctx, params)
			if err != nil {
				return nil, xerrors.Errorf("get playlist tracks failed: %w", err)
			}

			data, err := json.Marshal(response)
			if err != nil {
				return nil, xerrors.Errorf("failed to marshal response: %w", err)
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("get_currently_playing",
			mcp.WithDescription("Get the track currently being played on the user's account. Requires user-read-currently-playing scope."),
			mcp.WithString("market", mcp.Description("ISO 3166-1 alpha-2 country code")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			params := spotify.GetTheUsersCurrentlyPlayingTrackParams{}
			if v := request.GetString("market", ""); v != "" {
				params.Market = spotify.NewOptString(v)
			}

			response, err := c.GetTheUsersCurrentlyPlayingTrack(ctx, params)
			if err != nil {
				return nil, xerrors.Errorf("get currently playing failed: %w", err)
			}

			data, err := json.Marshal(response)
			if err != nil {
				return nil, xerrors.Errorf("failed to marshal response: %w", err)
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("get_recently_played",
			mcp.WithDescription("Get the current user's recently played tracks. Requires user-read-recently-played scope."),
			mcp.WithNumber("limit", mcp.Description("Maximum number of results (1-50, default 20)")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			params := spotify.GetRecentlyPlayedParams{}
			if v := request.GetInt("limit", 0); v > 0 {
				params.Limit = spotify.NewOptInt(v)
			}

			response, err := c.GetRecentlyPlayed(ctx, params)
			if err != nil {
				return nil, xerrors.Errorf("get recently played failed: %w", err)
			}

			data, err := json.Marshal(response)
			if err != nil {
				return nil, xerrors.Errorf("failed to marshal response: %w", err)
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}
