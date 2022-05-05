# go-messagefix

```shell
go get github.com/delthas/go-messagefix
```

go-messagefix offers an io.Reader that transforms RFC822 messages (.EML file contents) on-the-fly from another io.Reader.
The message is slightly transformed/fixed if it breaks the RFC822 spec so that strict implementations may accept it.

These transformations are best-effort heuristics and can include:
- closing any multiparts that are still open at EOF
- correctly indenting continuation headers that were not indented

## Example

```go
message := `Foo: Bar
Broken-Header:
"This_header_is_broken"
Content-Type: text/plain

Data
`
r := strings.NewReader(message)
fix := messagefix.NewReader(r)
io.Copy(os.Stdout, fix)
```

This prints:
```
Foo: Bar
Broken-Header:
 "This_header_is_broken"
Content-Type: text/plain

Data
```

The header continuation is properly indented.

## License

MIT
