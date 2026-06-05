package behavior

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

const SignatureCacheRelativePath = "behavior/signature.json"

var ErrSignatureCacheMiss = errors.New("behavior signature cache not found")

type SignatureCacheResult struct {
	Signature Signature `json:"signature"`
	CacheHit  bool      `json:"cache_hit"`
	TraceHash string    `json:"trace_hash,omitempty"`
	CachePath string    `json:"cache_path"`
}

func SignatureCachePath(runDir string) string {
	return filepath.Join(runDir, filepath.FromSlash(SignatureCacheRelativePath))
}

func ReadSignatureCache(runDir string) (Signature, error) {
	data, err := os.ReadFile(SignatureCachePath(runDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Signature{}, ErrSignatureCacheMiss
		}
		return Signature{}, err
	}
	var signature Signature
	if err := json.Unmarshal(data, &signature); err != nil {
		return Signature{}, err
	}
	return signature, nil
}

func WriteSignatureCache(runDir string, signature Signature) error {
	path := SignatureCachePath(runDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(signature, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func LoadOrBuildSignatureCache(runDir string, timeline Timeline, opts SignatureOptions) (SignatureCacheResult, error) {
	traceHash, err := ComputeRunTraceHash(runDir)
	if err != nil {
		return SignatureCacheResult{}, err
	}

	cachePath := SignatureCachePath(runDir)
	cached, err := ReadSignatureCache(runDir)
	if err == nil && signatureCacheMatches(cached, traceHash) {
		return SignatureCacheResult{
			Signature: cached,
			CacheHit:  true,
			TraceHash: traceHash,
			CachePath: cachePath,
		}, nil
	}
	if err != nil && !errors.Is(err, ErrSignatureCacheMiss) {
		return SignatureCacheResult{}, err
	}

	signature := buildSignatureFromTimelineAt(timeline, opts, time.Now().UTC(), traceHash)
	if err := WriteSignatureCache(runDir, signature); err != nil {
		return SignatureCacheResult{}, err
	}
	return SignatureCacheResult{
		Signature: signature,
		CacheHit:  false,
		TraceHash: traceHash,
		CachePath: cachePath,
	}, nil
}

func ComputeRunTraceHash(runDir string) (string, error) {
	file, err := os.Open(filepath.Join(runDir, "trace.ndjson"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func signatureCacheMatches(signature Signature, traceHash string) bool {
	if traceHash == "" {
		return true
	}
	return signature.SourceTraceHash == traceHash
}
