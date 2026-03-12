package application

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	videosProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "processor_videos_processed_total",
		Help: "Total de vídeos processados",
	}, []string{"status"})

	processingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "processor_video_processing_duration_seconds",
		Help:    "Duração do processamento de vídeo em segundos",
		Buckets: prometheus.DefBuckets,
	})

	framesExtracted = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "processor_frames_extracted",
		Help:    "Quantidade de frames extraídos por vídeo",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
	})
)
