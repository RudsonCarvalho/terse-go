# terse-go

Go implementation of the TERSE serialization format.

TERSE is a compact, human-readable data format that supports:
- Primitives: null (`~`), booleans (`T`/`F`), numbers, strings
- Inline and block objects
- Inline, block, and schema arrays

## Usage

```go
import terse "github.com/RudsonCarvalho/terse-go"

// Serialize
out, err := terse.Serialize(map[string]any{"name": "Alice", "age": float64(30)})

// Parse
val, err := terse.Parse(out)
```

## Supported types

| Go type          | TERSE         |
|------------------|---------------|
| `nil`            | `~`           |
| `bool` true      | `T`           |
| `bool` false     | `F`           |
| `float64`        | number        |
| `string`         | bare or quoted|
| `map[string]any` | object        |
| `[]any`          | array         |

## Format examples

```
# Inline object
{name:Alice age:30}

# Block object
name:Alice
age:30
active:T

# Inline array
[1 2 3]

# Schema array (homogeneous records)
#[id name score]
1 Alice 95
2 Bob   87
```

## License

MIT
