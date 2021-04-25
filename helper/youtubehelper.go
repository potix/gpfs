package helper

import (
        "context"
        "fmt"
        "google.golang.org/api/option"
        "google.golang.org/api/youtube/v3"
        "time"
)

const (
        timeout time.Duration = 20
)

type youtubeHelperOptions struct {
        verbose bool
}

func defaultYoutubeHelperOptions() *youtubeHelperOptions {
        return &youtubeHelperOptions{
                verbose: false,
        }
}

type YoutubeHelperOption func(*youtubeHelperOptions)

func YoutubeHelperVerbose(verbose bool) YoutubeHelperOption {
        return func(opts *youtubeHelperOptions) {
                opts.verbose = verbose
        }
}

type YoutubeHelper struct {
        verbose bool
        apiKey  string
        videoId string
}

func (y *YoutubeHelper) GetVideoTitle() (string, error) {
        ctx, cancel := context.WithTimeout(context.Background(), time.Second * timeout)
        defer cancel()
        youtubeService, err := youtube.NewService(ctx, option.WithAPIKey(y.apiKey))
        if err != nil {
                return "", fmt.Errorf("can not create youtube service: %w", err)
        }
        videosListCall := youtubeService.Videos.List([]string{"snippet"})
        videosListCall.Id(y.videoId)
        videoListResponse, err := videosListCall.Do()
        if err != nil {
                return "", fmt.Errorf("can not get videos (videoId = %v): %w", y.videoId, err)
        }
        if len(videoListResponse.Items) < 1 {
                return "", fmt.Errorf("no video (videoId = %v): %w", y.videoId, err)
        }
        return videoListResponse.Items[0].Snippet.Title, nil
}

func NewYoutubeHelper(apiKey string, videoId string, opts ...YoutubeHelperOption) *YoutubeHelper {
        baseOpts := defaultYoutubeHelperOptions()
        for _, opt := range opts {
                opt(baseOpts)
        }
        return &YoutubeHelper{
                verbose: baseOpts.verbose,
                apiKey:  apiKey,
                videoId: videoId,
        }
}
