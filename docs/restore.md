# restore

This package is setup to always attempt restore on start-up, but to restore a database from object storage outside of it, use litestream:


Set the credentials:

```shell
export AWS_ACCESS_KEY_ID=xxx
export AWS_SECRET_ACCESS_KEY=yyy
```

Run `litestream restore`:

```
litestream restore -o backup.db "s3://bucket/path?region=ber1"
```