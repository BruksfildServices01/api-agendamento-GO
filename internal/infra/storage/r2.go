package storage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/chai2010/webp"
	"golang.org/x/image/draw"

	// register decoders
	_ "image/jpeg"
	_ "image/png"
	_ "golang.org/x/image/webp"
)

// Dimensions for each asset type.
const (
	serviceImageW = 800
	serviceImageH = 600
	productImageW = 400
	productImageH = 400
	profilePhotoW = 1200
	profilePhotoH = 630
)

type AssetKind string

const (
	KindServiceImage AssetKind = "services"
	KindProductImage AssetKind = "products"
	KindProfilePhoto AssetKind = "profile"
)

type R2Service struct {
	client    *s3.Client
	bucket    string
	publicURL string
}

func NewR2Service(accountID, accessKeyID, secretKey, bucket, publicURL string) *R2Service {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, ""),
	})

	return &R2Service{
		client:    client,
		bucket:    bucket,
		publicURL: strings.TrimRight(publicURL, "/"),
	}
}

// Upload converts raw image bytes to WebP, resizes to the appropriate dimensions
// for the given kind, uploads to R2, and returns the public URL.
func (s *R2Service) Upload(ctx context.Context, kind AssetKind, objectKey string, raw []byte) (string, error) {
	maxW, maxH := dimensionsFor(kind)

	webpBytes, err := convertToWebP(raw, maxW, maxH)
	if err != nil {
		return "", fmt.Errorf("image conversion: %w", err)
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(webpBytes),
		ContentType: aws.String("image/webp"),
	})
	if err != nil {
		return "", fmt.Errorf("r2 upload: %w", err)
	}

	return s.publicURL + "/" + objectKey, nil
}

// Delete removes an object from R2 by its key.
func (s *R2Service) Delete(ctx context.Context, objectKey string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	return err
}

// KeyFromURL extracts the object key from a full public URL.
func (s *R2Service) KeyFromURL(url string) string {
	return strings.TrimPrefix(url, s.publicURL+"/")
}

// ──────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────

func dimensionsFor(kind AssetKind) (int, int) {
	switch kind {
	case KindServiceImage:
		return serviceImageW, serviceImageH
	case KindProductImage:
		return productImageW, productImageH
	case KindProfilePhoto:
		return profilePhotoW, profilePhotoH
	default:
		return 800, 600
	}
}

func convertToWebP(raw []byte, maxW, maxH int) ([]byte, error) {
	src, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		// Try jpeg and png explicitly as fallback.
		if src, err = tryDecode(raw); err != nil {
			return nil, fmt.Errorf("unsupported format %q: %w", format, err)
		}
	}

	resized := resizeFit(src, maxW, maxH)

	var buf bytes.Buffer
	if err := webp.Encode(&buf, resized, &webp.Options{Lossless: false, Quality: 85}); err != nil {
		return nil, fmt.Errorf("webp encode: %w", err)
	}
	return buf.Bytes(), nil
}

func tryDecode(raw []byte) (image.Image, error) {
	if img, err := jpeg.Decode(bytes.NewReader(raw)); err == nil {
		return img, nil
	}
	return png.Decode(bytes.NewReader(raw))
}

// resizeFit resizes src to fit within maxW×maxH preserving aspect ratio.
func resizeFit(src image.Image, maxW, maxH int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if srcW <= maxW && srcH <= maxH {
		return src
	}

	scaleW := float64(maxW) / float64(srcW)
	scaleH := float64(maxH) / float64(srcH)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}

	dstW := int(float64(srcW) * scale)
	dstH := int(float64(srcH) * scale)
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}
