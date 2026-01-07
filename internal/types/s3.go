package types

// S3ObjectInfo identifies an S3 object by bucket and key.
type S3ObjectInfo struct {
	Bucket string
	Key    string
}
