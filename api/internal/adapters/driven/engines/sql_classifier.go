package engines

import (
	"strings"
	"unicode"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

func (manager *Manager) ClassifyStatement(engine entity.Engine, sqlText string) (ports.QueryStatement, error) {
	if !engine.Valid() {
		return ports.QueryStatement{}, apperror.NewUnsupported("database engine is not supported", nil)
	}
	tokens, err := statementTokens(sqlText)
	if err != nil {
		return ports.QueryStatement{}, err
	}
	return classifyTokens(tokens), nil
}

func classifyTokens(tokens []string) ports.QueryStatement {
	if len(tokens) == 0 {
		return ports.QueryStatement{Type: dto.QueryStatementUnknown}
	}
	keyword := tokens[0]
	if keyword == "with" {
		statement := classifyTokens([]string{withStatementKeyword(tokens[1:])})
		if statement.ReadOnly && containsWriteStatement(tokens[1:]) {
			statement.ReadOnly = false
		}
		return statement
	}
	if keyword == "explain" {
		return classifyExplainTokens(tokens[1:])
	}
	switch keyword {
	case "select", "values", "table":
		return ports.QueryStatement{Type: dto.QueryStatementSelect, ReadOnly: true}
	case "insert", "replace":
		return ports.QueryStatement{Type: dto.QueryStatementInsert}
	case "update":
		return ports.QueryStatement{Type: dto.QueryStatementUpdate}
	case "delete":
		return ports.QueryStatement{Type: dto.QueryStatementDelete}
	case "merge":
		return ports.QueryStatement{Type: dto.QueryStatementMerge}
	case "create", "alter", "drop", "truncate", "comment", "rename", "grant", "revoke":
		return ports.QueryStatement{Type: dto.QueryStatementDDL}
	case "begin", "start", "commit", "rollback", "savepoint", "release":
		return ports.QueryStatement{Type: dto.QueryStatementTransaction}
	case "show", "describe", "desc":
		return ports.QueryStatement{Type: dto.QueryStatementUtility, ReadOnly: true}
	case "set", "reset", "use", "call", "do", "load", "lock", "unlock", "vacuum", "analyze", "refresh", "reindex", "cluster":
		return ports.QueryStatement{Type: dto.QueryStatementUtility}
	case "copy":
		return ports.QueryStatement{Type: dto.QueryStatementUtility, ReadOnly: copyIsReadOnly(tokens)}
	default:
		return ports.QueryStatement{Type: dto.QueryStatementUnknown}
	}
}

func containsWriteStatement(tokens []string) bool {
	for _, token := range tokens {
		if token == "insert" || token == "replace" || token == "update" || token == "delete" || token == "merge" {
			return true
		}
	}
	return false
}

func classifyExplainTokens(tokens []string) ports.QueryStatement {
	analyze := false
	depth := 0
	for index, token := range tokens {
		switch token {
		case "(":
			depth++
		case ")":
			if depth > 0 {
				depth--
			}
		case "analyze", "analyse":
			analyze = true
		}
		if depth == 0 && isStatementKeyword(token) {
			inner := classifyTokens(tokens[index:])
			readOnly := true
			if analyze {
				readOnly = inner.ReadOnly
			}
			return ports.QueryStatement{Type: dto.QueryStatementUtility, ReadOnly: readOnly}
		}
	}
	return ports.QueryStatement{Type: dto.QueryStatementUtility, ReadOnly: !analyze}
}

func withStatementKeyword(tokens []string) string {
	depth := 0
	for _, token := range tokens {
		switch token {
		case "(":
			depth++
			continue
		case ")":
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 && isDataStatementKeyword(token) {
			return token
		}
	}
	return "with"
}

func isStatementKeyword(token string) bool {
	return isDataStatementKeyword(token) || token == "create" || token == "alter" || token == "drop" || token == "truncate" || token == "call" || token == "do" || token == "copy"
}

func isDataStatementKeyword(token string) bool {
	return token == "select" || token == "values" || token == "table" || token == "insert" || token == "replace" || token == "update" || token == "delete" || token == "merge"
}

func copyIsReadOnly(tokens []string) bool {
	for _, token := range tokens[1:] {
		if token == "from" {
			return false
		}
		if token == "to" {
			return true
		}
	}
	return false
}

func statementReturnsRows(sqlText string, statement ports.QueryStatement) bool {
	if statement.Type == dto.QueryStatementSelect {
		return true
	}
	tokens, err := statementTokens(sqlText)
	if err != nil || len(tokens) == 0 {
		return false
	}
	first := tokens[0]
	if first == "show" || first == "describe" || first == "desc" || first == "explain" || first == "call" {
		return true
	}
	if statement.Type == dto.QueryStatementInsert || statement.Type == dto.QueryStatementUpdate || statement.Type == dto.QueryStatementDelete || statement.Type == dto.QueryStatementMerge {
		for _, token := range tokens {
			if token == "returning" {
				return true
			}
		}
	}
	return false
}

func statementTokens(sqlText string) ([]string, error) {
	tokens := make([]string, 0, 32)
	for index := 0; index < len(sqlText); {
		character := sqlText[index]
		if unicode.IsSpace(rune(character)) {
			index++
			continue
		}
		if character == '-' && index+1 < len(sqlText) && sqlText[index+1] == '-' || character == '#' {
			for index < len(sqlText) && sqlText[index] != '\n' {
				index++
			}
			continue
		}
		if character == '/' && index+1 < len(sqlText) && sqlText[index+1] == '*' {
			end, ok := skipBlockComment(sqlText, index)
			if !ok {
				return nil, apperror.NewValidation("SQL contains an unterminated comment", nil)
			}
			index = end
			continue
		}
		if character == '\'' {
			end, ok := skipQuoted(sqlText, index, character, true)
			if !ok {
				return nil, apperror.NewValidation("SQL contains an unterminated string", nil)
			}
			index = end
			continue
		}
		if character == '"' || character == '`' {
			end, ok := skipQuoted(sqlText, index, character, false)
			if !ok {
				return nil, apperror.NewValidation("SQL contains an unterminated identifier", nil)
			}
			index = end
			tokens = append(tokens, "identifier")
			continue
		}
		if character == '$' {
			if end, ok := skipDollarQuoted(sqlText, index); ok {
				index = end
				continue
			}
		}
		if isWordByte(character) {
			start := index
			for index < len(sqlText) && isWordByte(sqlText[index]) {
				index++
			}
			tokens = append(tokens, strings.ToLower(sqlText[start:index]))
			continue
		}
		if character == ';' {
			if hasSQLAfter(sqlText, index+1) {
				return nil, apperror.NewValidation("execute one SQL statement at a time", nil)
			}
			index++
			continue
		}
		if character == '(' || character == ')' || character == ',' {
			tokens = append(tokens, string(character))
		}
		index++
	}
	if len(tokens) == 0 {
		return nil, apperror.NewValidation("SQL is required", nil)
	}
	return tokens, nil
}

func skipBlockComment(value string, start int) (int, bool) {
	depth := 1
	for index := start + 2; index < len(value); {
		if index+1 < len(value) && value[index] == '/' && value[index+1] == '*' {
			depth++
			index += 2
			continue
		}
		if index+1 < len(value) && value[index] == '*' && value[index+1] == '/' {
			depth--
			index += 2
			if depth == 0 {
				return index, true
			}
			continue
		}
		index++
	}
	return len(value), false
}

func skipQuoted(value string, start int, quote byte, backslash bool) (int, bool) {
	for index := start + 1; index < len(value); index++ {
		if backslash && value[index] == '\\' && index+1 < len(value) {
			index++
			continue
		}
		if value[index] == quote {
			if index+1 < len(value) && value[index+1] == quote {
				index++
				continue
			}
			return index + 1, true
		}
	}
	return len(value), false
}

func skipDollarQuoted(value string, start int) (int, bool) {
	endTag := start + 1
	for endTag < len(value) && (value[endTag] == '_' || value[endTag] >= 'a' && value[endTag] <= 'z' || value[endTag] >= 'A' && value[endTag] <= 'Z' || value[endTag] >= '0' && value[endTag] <= '9') {
		endTag++
	}
	if endTag >= len(value) || value[endTag] != '$' {
		return start, false
	}
	tag := value[start : endTag+1]
	closing := strings.Index(value[endTag+1:], tag)
	if closing < 0 {
		return start, false
	}
	return endTag + 1 + closing + len(tag), true
}

func hasSQLAfter(value string, start int) bool {
	remaining := strings.TrimSpace(value[start:])
	for remaining != "" {
		if strings.HasPrefix(remaining, "--") || strings.HasPrefix(remaining, "#") {
			if newline := strings.IndexByte(remaining, '\n'); newline >= 0 {
				remaining = strings.TrimSpace(remaining[newline+1:])
				continue
			}
			return false
		}
		if strings.HasPrefix(remaining, "/*") {
			end, ok := skipBlockComment(remaining, 0)
			if !ok {
				return true
			}
			remaining = strings.TrimSpace(remaining[end:])
			continue
		}
		if strings.HasPrefix(remaining, ";") {
			remaining = strings.TrimSpace(remaining[1:])
			continue
		}
		return true
	}
	return false
}

func isWordByte(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}
