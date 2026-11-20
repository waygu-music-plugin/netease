package netease

import (
	"context"
	"errors"

	"github.com/waygu-music-plugin/netease/internal/client"
	"github.com/waygu-music-plugin/sdk/pluginapi"
)

func (*adapter) OwnsTrack(tr *pluginapi.Track) bool {
	// NetEase tracks persisted before plugins had no source field.
	return tr.Source == ""
}

func (p *adapter) PlaybackMedia() pluginapi.PlaybackMedia {
	return pluginapi.PlaybackMedia{
		Preferences: func() pluginapi.AudioPreferences {
			s := p.ui.preferences
			return pluginapi.AudioPreferences{Quality: s.Quality, Normalize: s.Normalize}
		},
		Stream: func(ctx context.Context, tr *pluginapi.Track, level string) (*pluginapi.Stream, error) {
			return netease.SongURL(ctx, tr.ID, level)
		},
		CastSource: func(ctx context.Context, tr *pluginapi.Track, format string) (*pluginapi.Stream, pluginapi.SegmentedSource, error) {
			if format != "mp3" {
				return nil, nil, nil
			}
			st, err := netease.SongURL(ctx, tr.ID, "standard")
			return st, nil, err
		},
		Cover: func(ctx context.Context, tr *pluginapi.Track) ([]byte, error) {
			return netease.Fetch(ctx, netease.Sized(tr.Album.PicURL, 600))
		},
		Lyrics: func(ctx context.Context, tr *pluginapi.Track) (string, string, string, error) {
			return netease.Lyrics(ctx, tr.ID)
		},
		ErrorMessage: func(tr *pluginapi.Track, err error) string {
			switch {
			case errors.Is(err, netease.ErrLoginRequired):
				return "网易云音乐登录已失效，请重新登录"
			case errors.Is(err, netease.ErrNetworkRestricted):
				return "网易云音乐限制了当前网络的播放请求，请稍后重试"
			case errors.Is(err, netease.ErrPreviewOnly):
				if tr.ProgramID != 0 {
					return "这期节目目前只能试听"
				}
				return "这首歌目前只能试听"
			case errors.Is(err, netease.ErrUnavailable):
				if tr.ProgramID != 0 {
					return "网易云音乐未提供这期节目的完整音源"
				}
				return "网易云音乐未提供这首歌的完整音源"
			default:
				return ""
			}
		},
	}
}
