proto-golint
===

> Linter for Go code that uses [protobuf messages](https://developers.google.com/protocol-buffers/).

# Getters

Reports reading fields of proto message structs without the getter:

```
internal/test/proto.go:33:3: getters: proto message field read without getter: t.Embedded (proto-golint)
                t.Embedded,
```

Supports `--fix` flag to apply the suggested fix directly.

# Why?

This is mostly a fun project to learn how to write a linter for Go. **[Fatih Arslan's][fatih] blog was a [tremendous][analysis] [help][fix] in doing that!**

[fatih]: https://arslan.io/
[analysis]: https://arslan.io/2019/06/13/using-go-analysis-to-write-a-custom-linter/
[fix]: https://arslan.io/2020/07/07/using-go-analysis-to-fix-your-source-code/