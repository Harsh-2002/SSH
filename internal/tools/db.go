package tools

import (
	"context"
	"fmt"

	"ssh-mcp/internal/ssh"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerDBTools registers database query tools.
func registerDBTools(s *server.MCPServer, pool *ssh.Pool) {
	// db_query
	s.AddTool(
		mcp.NewTool("db_query",
			mcp.WithDescription("Execute SQL/CQL/MongoDB query inside a database container"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Docker container name running the database")),
			mcp.WithString("db_type", mcp.Required(), mcp.Description("Database type: postgres, mysql, scylladb, cassandra, mongodb")),
			mcp.WithString("query", mcp.Required(), mcp.Description("Query to execute")),
			mcp.WithString("database", mcp.Description("Database/keyspace name")),
			mcp.WithString("username", mcp.Description("Database username")),
			mcp.WithString("password", mcp.Description("Database password")),
			mcp.WithNumber("timeout", mcp.Description("Query timeout in seconds (default: 60)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createDBQueryHandler(pool),
	)

	// db_schema
	s.AddTool(
		mcp.NewTool("db_schema",
			mcp.WithDescription("Get database schema (tables/collections list)"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Docker container name")),
			mcp.WithString("db_type", mcp.Required(), mcp.Description("Database type: postgres, mysql, scylladb, cassandra, mongodb")),
			mcp.WithString("database", mcp.Description("Database/keyspace name")),
			mcp.WithString("username", mcp.Description("Database username")),
			mcp.WithString("password", mcp.Description("Database password")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createDBSchemaHandler(pool),
	)

	// list_db_containers
	s.AddTool(
		mcp.NewTool("list_db_containers",
			mcp.WithDescription("Find Docker containers that look like databases"),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createListDBContainersHandler(pool),
	)
}

func createDBQueryHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		dbType, _ := req.RequireString("db_type")
		query, _ := req.RequireString("query")
		database := req.GetString("database", "")
		username := req.GetString("username", "")
		password := req.GetString("password", "")
		timeout := req.GetInt("timeout", 60)
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var cmd string
		switch dbType {
		case "postgres":
			user := "postgres"
			if username != "" {
				user = username
			}
			db := database
			if db == "" {
				db = "postgres"
			}
			// Use PGPASSWORD env variable and echo query through stdin
			cmd = fmt.Sprintf("docker exec -e PGPASSWORD=%s %s timeout %d psql -U %s -d %s -c %s 2>&1",
				shellQuote(password), shellQuote(container), timeout, shellQuote(user), shellQuote(db), shellQuote(query))

		case "mysql":
			user := "root"
			if username != "" {
				user = username
			}
			cmd = fmt.Sprintf("docker exec %s timeout %d mysql -u%s", shellQuote(container), timeout, shellQuote(user))
			if password != "" {
				cmd += fmt.Sprintf(" -p%s", shellQuote(password))
			}
			if database != "" {
				cmd += fmt.Sprintf(" %s", shellQuote(database))
			}
			cmd += fmt.Sprintf(" -e %s 2>&1", shellQuote(query))

		case "scylladb", "cassandra":
			cmd = fmt.Sprintf("docker exec %s timeout %d cqlsh", shellQuote(container), timeout)
			if username != "" {
				cmd += fmt.Sprintf(" -u %s", shellQuote(username))
			}
			if password != "" {
				cmd += fmt.Sprintf(" -p %s", shellQuote(password))
			}
			cmd += fmt.Sprintf(" -e %s 2>&1", shellQuote(query))

		case "mongodb":
			db := database
			if db == "" {
				db = "admin"
			}
			cmd = fmt.Sprintf("docker exec %s timeout %d mongosh --quiet %s", shellQuote(container), timeout, shellQuote(db))
			if username != "" && password != "" {
				cmd += fmt.Sprintf(" -u %s -p %s --authenticationDatabase admin", shellQuote(username), shellQuote(password))
			}
			cmd += fmt.Sprintf(" --eval %s 2>&1", shellQuote(query))

		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unsupported database type: %s. Supported: postgres, mysql, scylladb, cassandra, mongodb", dbType)), nil
		}

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createDBSchemaHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		dbType, _ := req.RequireString("db_type")
		database := req.GetString("database", "")
		username := req.GetString("username", "")
		password := req.GetString("password", "")
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var query string
		switch dbType {
		case "postgres":
			query = "\\dt"
		case "mysql":
			query = "SHOW TABLES;"
		case "scylladb", "cassandra":
			if database != "" {
				query = fmt.Sprintf("DESCRIBE KEYSPACE %s;", database)
			} else {
				query = "DESCRIBE KEYSPACES;"
			}
		case "mongodb":
			query = "db.getCollectionNames()"
		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unsupported database type: %s", dbType)), nil
		}

		// Build command directly
		var cmd string
		switch dbType {
		case "postgres":
			user := "postgres"
			if username != "" {
				user = username
			}
			db := database
			if db == "" {
				db = "postgres"
			}
			cmd = fmt.Sprintf("docker exec -e PGPASSWORD=%s %s psql -U %s -d %s -c %s 2>&1",
				shellQuote(password), shellQuote(container), shellQuote(user), shellQuote(db), shellQuote(query))
		case "mysql":
			user := "root"
			if username != "" {
				user = username
			}
			cmd = fmt.Sprintf("docker exec %s mysql -u%s", shellQuote(container), shellQuote(user))
			if password != "" {
				cmd += fmt.Sprintf(" -p%s", shellQuote(password))
			}
			if database != "" {
				cmd += fmt.Sprintf(" %s", shellQuote(database))
			}
			cmd += fmt.Sprintf(" -e %s 2>&1", shellQuote(query))
		case "scylladb", "cassandra":
			cmd = fmt.Sprintf("docker exec %s cqlsh", shellQuote(container))
			if username != "" {
				cmd += fmt.Sprintf(" -u %s", shellQuote(username))
			}
			if password != "" {
				cmd += fmt.Sprintf(" -p %s", shellQuote(password))
			}
			cmd += fmt.Sprintf(" -e %s 2>&1", shellQuote(query))
		case "mongodb":
			db := database
			if db == "" {
				db = "admin"
			}
			cmd = fmt.Sprintf("docker exec %s mongosh --quiet %s", shellQuote(container), shellQuote(db))
			if username != "" && password != "" {
				cmd += fmt.Sprintf(" -u %s -p %s --authenticationDatabase admin", shellQuote(username), shellQuote(password))
			}
			cmd += fmt.Sprintf(" --eval %s 2>&1", shellQuote(query))
		}

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createListDBContainersHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cmd := `docker ps --format '{{.Names}}|{{.Image}}' | while read line; do
  name=$(echo "$line" | cut -d'|' -f1)
  image=$(echo "$line" | cut -d'|' -f2)
  case "$image" in
    *postgres*) echo "$name|$image|postgres" ;;
    *mysql*|*mariadb*) echo "$name|$image|mysql" ;;
    *scylla*) echo "$name|$image|scylladb" ;;
    *cassandra*) echo "$name|$image|cassandra" ;;
    *mongo*) echo "$name|$image|mongodb" ;;
    *redis*) echo "$name|$image|redis" ;;
  esac
done 2>/dev/null`

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if trimOutput(output) == "" {
			return mcp.NewToolResultText("No database containers found"), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}
