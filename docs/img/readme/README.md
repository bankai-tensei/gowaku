# Pictures of the README

`showcase-light.png` and `showcase-dark.png` are screenshots of an application built with gowaku. The rest are drawn by the examples themselves, on the GPU, at a scale of 2 on Windows. To make those again:

```bash
go run ./examples/tategaki -png tategaki.png
go run ./examples/notes -shot palette-light.png -scene palette
go run ./examples/notes -shot palette-dark.png -scene palette -dark
go run ./examples/notes -shot notes-vertical-light.png -scene vertical
go run ./examples/notes -shot notes-vertical-dark.png -scene vertical -dark
go run ./examples/glass -png glass.png
go run ./examples/gallery -png desktop-light.png -state desktop
go run ./examples/gallery -png desktop-dark.png -state desktop -dark
go run ./examples/scripts -png scripts.png
```
