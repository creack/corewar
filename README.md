# Corewar

## Live Demo

[https://creack.github.io/corewar](https://creack.github.io/corewar)

## Window mode

```sh
go run go.creack.net/corewar/vm@latest

# Or simply
go run ./vm
```

## WASM

### One liner

```sh
# Without Docker
env -i HOME=${HOME} PATH=${PATH} go tool wasmserve ./vm

# With Docker
make
```

### Details

Clone this repo:

```sh
git clone https://github.com/creack/corewar
cd corewar
```

Run:

```sh
env -i HOME=${HOME} PATH=${PATH} go tool wasmserve ./vm
```

For development, `wasmer` exposes an endpoint to do live reload.

I recommend [reflex](https://github.com/cespare/reflex).

With `wasmer` running, run:

```sh
go tool reflex curl -v http://localhost:8080/_notify
```

## Docker

A Dockerfile is provided to build and run the WASM version.

### Build

```sh
docker build -t corewar .
```

### Regular run

To run the image, make sure to have:

- `--rm` to avoid pollution
- `-it` so the app receives signals
- `-p` to expose the port 8080

Any changes to the code will require to re-build the image.

```sh
docker run --rm -p 8080:8080 -it corewar wasmserve .
```

You can then access the WASM page at the Docker ip on port 8080. If in doubt about the IP, it is likely localhost.

### Development run

For development, you can add `-v $(pwd):/app` to mount the local directory in the Docker container, the server will hot-reload when file changes.

```sh
docker run --rm -p 8080:8080 -it -v $(pwd):/app corewar
```
