package compiler

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"sort"
	"strings"

	"pads-v3/internal/storage"
)

// IngestResult résume l'ingestion d'un fichier.
type IngestResult struct {
	FilePath   string
	NodesAdded int
	EdgesAdded int
}

// nodeInfo représente un nœud extrait.
type nodeInfo struct {
	ID           string
	Type         string
	Signature    string
	ReceiverType string
	ArgsTypes    []string
}

// edgeInfo représente une arête extraite.
type edgeInfo struct {
	Source   string
	Target   string
	Relation string
}

// symbolKey est une clé composite stable pour un symbole local.
type symbolKey struct {
	receiver string
	name     string
}

// IngestFile parse un fichier Go et insère son graphe L1 dans la base.
func IngestFile(db *storage.DB, filePath string) (*IngestResult, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}

	// Calculer le hash du fichier
	fileHash, err := computeFileHash(filePath)
	if err != nil {
		return nil, fmt.Errorf("file hash %s: %w", filePath, err)
	}

	pkgName := file.Name.Name
	fileNodeID := fmt.Sprintf("%s/%s", pkgName, filePath)

	// Extraction des symboles locaux.
	canonicalID := make(map[symbolKey]string)

	// Première passe : collecter les symboles.
	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			name := decl.Name.Name
			if decl.Recv == nil || len(decl.Recv.List) == 0 {
				id := fmt.Sprintf("%s.%s", pkgName, name)
				canonicalID[symbolKey{name: name}] = id
			} else {
				recvType := extractReceiverType(decl.Recv.List[0].Type)
				id := fmt.Sprintf("%s.%s.%s", pkgName, recvType, name)
				canonicalID[symbolKey{receiver: recvType, name: name}] = id
			}
		}
		return true
	})

	var nodes []nodeInfo
	var edges []edgeInfo

	// Deuxième passe : extraction des nœuds et arcs.
	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.TypeSpec:
			var nodeType string
			switch decl.Type.(type) {
			case *ast.StructType:
				nodeType = "struct"
			case *ast.InterfaceType:
				nodeType = "interface"
			default:
				return true
			}
			id := fmt.Sprintf("%s.%s", pkgName, decl.Name.Name)
			nodes = append(nodes, nodeInfo{ID: id, Type: nodeType, Signature: decl.Name.Name})
			edges = append(edges, edgeInfo{Source: fileNodeID, Target: id, Relation: "CONTAINS"})

		case *ast.FuncDecl:
			funcName := decl.Name.Name
			var id string
			receiver := ""
			args := []string{}
			for _, param := range decl.Type.Params.List {
				args = append(args, typeToString(param.Type))
			}
			sort.Strings(args)

			if decl.Recv == nil || len(decl.Recv.List) == 0 {
				id = fmt.Sprintf("%s.%s", pkgName, funcName)
			} else {
				recvType := extractReceiverType(decl.Recv.List[0].Type)
				receiver = recvType
				id = fmt.Sprintf("%s.%s.%s", pkgName, recvType, funcName)
			}

			nodes = append(nodes, nodeInfo{ID: id, Type: "func", Signature: funcName, ReceiverType: receiver, ArgsTypes: args})
			edges = append(edges, edgeInfo{Source: fileNodeID, Target: id, Relation: "CONTAINS"})

			// Extraction des CALLS
			if decl.Body != nil {
				ast.Inspect(decl.Body, func(call ast.Node) bool {
					callExpr, ok := call.(*ast.CallExpr)
					if !ok {
						return true
					}
					switch fun := callExpr.Fun.(type) {
					case *ast.Ident:
						callee := fun.Name
						if resolvedID, exists := canonicalID[symbolKey{name: callee}]; exists {
							edges = append(edges, edgeInfo{
								Source:   id,
								Target:   resolvedID,
								Relation: "CALLS",
							})
						} else {
							edges = append(edges, edgeInfo{
								Source:   id,
								Target:   fmt.Sprintf("unresolved:%s", callee),
								Relation: "CALLS",
							})
						}
					}
					return true
				})
			}
		}
		return true
	})

	// IMPORTS
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if imp.Name != nil && imp.Name.Name == "_" {
			continue
		}
		edges = append(edges, edgeInfo{Source: fileNodeID, Target: path, Relation: "IMPORTS"})
	}

	// Normalisation et dédoublonnage
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		if edges[i].Target != edges[j].Target {
			return edges[i].Target < edges[j].Target
		}
		return edges[i].Relation < edges[j].Relation
	})

	uniqueNodes := deduplicateNodes(nodes)
	uniqueEdges := deduplicateEdges(edges)

	// Insertion
	err = db.ClearFileNodes(filePath)
	if err != nil {
		return nil, fmt.Errorf("clear file nodes: %w", err)
	}

	nodesAdded := 0
	for _, nd := range uniqueNodes {
		hash, err := computeSignatureHash(nd)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", nd.ID, err)
		}
		err = db.InsertNode(nd.ID, nd.Type, filePath, hash, fileHash)
		if err != nil {
			return nil, fmt.Errorf("insert node %s: %w", nd.ID, err)
		}
		nodesAdded++
	}

	edgesAdded := 0
	for _, ed := range uniqueEdges {
		err = db.InsertEdge(ed.Source, ed.Target, ed.Relation)
		if err != nil {
			return nil, fmt.Errorf("insert edge %s->%s: %w", ed.Source, ed.Target, err)
		}
		edgesAdded++
	}

	return &IngestResult{FilePath: filePath, NodesAdded: nodesAdded, EdgesAdded: edgesAdded}, nil
}

func computeFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// extractReceiverType retourne le type du receveur exactement tel qu'il apparaît dans l'AST.
func extractReceiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return "*" + ident.Name
		}
	}
	return "unknown"
}

// computeSignatureHash calcule un SHA-256 déterministe pour un nœud.
func computeSignatureHash(nd nodeInfo) (string, error) {
	clean := func(s string) string { return strings.TrimSpace(s) }

	sortedArgs := make([]string, len(nd.ArgsTypes))
	copy(sortedArgs, nd.ArgsTypes)
	sort.Strings(sortedArgs)
	if sortedArgs == nil {
		sortedArgs = []string{}
	}

	canon := struct {
		NodeType     string   `json:"nodetype"`
		Name         string   `json:"name"`
		ReceiverType string   `json:"receiver_type,omitempty"`
		ArgsTypes    []string `json:"args_types,omitempty"`
	}{
		NodeType:     clean(nd.Type),
		Name:         clean(nd.Signature),
		ReceiverType: clean(nd.ReceiverType),
		ArgsTypes:    sortedArgs,
	}

	jsonBytes, err := json.Marshal(canon)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(jsonBytes)
	return fmt.Sprintf("%x", h), nil
}

// typeToString retourne une représentation simplifiée d'un type.
func typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeToString(t.X)
	case *ast.SelectorExpr:
		return typeToString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + typeToString(t.Elt)
		}
		return "array"
	default:
		return "unknown"
	}
}

func deduplicateNodes(nodes []nodeInfo) []nodeInfo {
	seen := make(map[string]bool)
	result := make([]nodeInfo, 0, len(nodes))
	for _, nd := range nodes {
		if !seen[nd.ID] {
			result = append(result, nd)
			seen[nd.ID] = true
		}
	}
	return result
}

func deduplicateEdges(edges []edgeInfo) []edgeInfo {
	seen := make(map[string]bool)
	result := make([]edgeInfo, 0, len(edges))
	for _, ed := range edges {
		key := ed.Source + "|" + ed.Target + "|" + ed.Relation
		if !seen[key] {
			result = append(result, ed)
			seen[key] = true
		}
	}
	return result
}
