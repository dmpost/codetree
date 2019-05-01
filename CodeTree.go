package codetree

import (
	"fmt"
	"io"
	"sync"
)

// Pool for CodeTree objects
var codeTreePool = sync.Pool{
	New: func() interface{} {
		return &CodeTree{}
	},
}

// Pool for []byte objects
var byteSlicePool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 4096)
	},
}

// CodeTree ...
type CodeTree struct {
	Line     string
	Children []*CodeTree
	Parent   *CodeTree
	Indent   int
}

// Close sends the tree and all of its children back to the memory pool.
// The resources are therefore freed up and the tree object should not be
// used after the final Close() call anymore.
func (tree *CodeTree) Close() {
	for _, child := range tree.Children {
		child.Close()
	}

	tree.Children = nil
	codeTreePool.Put(tree)
}

// New returns a tree structure if you feed it with indentantion based source code.
func New(reader io.Reader) (*CodeTree, error) {
	ast := codeTreePool.Get().(*CodeTree)
	ast.Indent = -1
	ast.Line = ""
	ast.Parent = nil

	var remains []byte
	var line []byte

	block := ast
	lastNode := ast
	lineStart := 0
	eof := false
	pooledBuffer := byteSlicePool.Get().([]byte)
	defer byteSlicePool.Put(pooledBuffer)
	buffer := pooledBuffer

	for {
		n, err := reader.Read(pooledBuffer)

		if err == io.EOF {
			eof = true
			err = nil

			buffer = append(remains, pooledBuffer[:n]...)
			buffer = append(buffer, '\n')
			n = len(buffer)
			remains = nil
		}

		for i := 0; i < n; i++ {
			if buffer[i] != '\n' {
				continue
			}

			// Ignore completely empty lines
			if i == lineStart {
				lineStart++
				continue
			}

			// Get the line
			if i > 0 && buffer[i-1] == '\r' {
				// Windows line endings
				line = buffer[lineStart : i-1]
			} else {
				// Unix line endings
				line = buffer[lineStart:i]
			}

			if remains != nil {
				line = append(remains, line...)
				remains = remains[:0]
			}

			lineStart = i + 1

			// Count indentation
			indent := 0

			for indent < len(line) {
				if line[indent] != '\t' {
					break
				}

				indent++
			}

			// Ignore empty lines (only tabs)
			if indent == len(line) {
				continue
			}

			// Cut off indentation prefix
			if indent != 0 {
				line = line[indent:]
			}

			if indent == block.Indent+1 {
				// OK
			} else if indent == block.Indent+2 {
				block = lastNode
			} else if indent <= block.Indent {
				for {
					block = block.Parent

					if block.Indent == indent-1 {
						break
					}
				}
			} else if indent > block.Indent+2 {
				return nil, fmt.Errorf("Invalid indentation on line: %s", line)
			}

			node := codeTreePool.Get().(*CodeTree)
			node.Line = string(line)
			node.Indent = indent
			node.Parent = block
			lastNode = node
			block.Children = append(block.Children, node)
		}

		if eof {
			return ast, nil
		}

		if err != nil {
			return nil, err
		}

		if lineStart < n {
			remains = append(remains, buffer[lineStart:n]...)
		}

		lineStart = 0
	}
}
