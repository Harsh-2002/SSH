import { ToolCategory, ConfigItem, InstallMethod, NavItem } from './types';

export const NAV_ITEMS: NavItem[] = [
  { label: 'Start', href: '#quickstart' },
  { label: 'How it Works', href: '#how-it-works' },
  { label: 'Screenshots', href: '#screenshots' },
  { label: 'Tools', href: '#tools' },
  { label: 'Config', href: '#configuration' },
];

export const INSTALL_METHODS: InstallMethod[] = [
  {
    id: 'docker',
    label: 'Docker CLI',
    commands: `# Run with default settings (HTTP port 8000)
docker run -d \\
  --name ssh-mcp \\
  -p 8000:8000 \\
  -v ssh-mcp-data:/data \\
  firstfinger/ssh-mcp:latest

# Custom Port & Debug Mode
docker run -d \\
  --name ssh-mcp \\
  -e PORT=9090 \\
  -e SSH_MCP_DEBUG=true \\
  -p 9090:9090 \\
  -v ssh-mcp-data:/data \\
  firstfinger/ssh-mcp:latest`,
    note: 'Distroless production image (11MB)'
  },
  {
    id: 'compose',
    label: 'Docker Compose',
    commands: `services:
  ssh-mcp:
    image: firstfinger/ssh-mcp:latest
    container_name: ssh-mcp
    ports:
      - "8000:8000"
    environment:
      - PORT=8000
    volumes:
      - ssh-data:/data
    restart: unless-stopped

volumes:
  ssh-data:`,
    note: 'Save as docker-compose.yml'
  },
  {
    id: 'local',
    label: 'Go Binary',
    commands: `# Build from source
git clone https://github.com/Harsh-2002/SSH-MCP.git
cd SSH-MCP
go build -o ssh-mcp ./cmd/server

# Run (Defaults using HTTP :8000)
./ssh-mcp

# Run with custom configuration
PORT=9090 SSH_MCP_DEBUG=true ./ssh-mcp`,
    note: 'Zero dependencies. Single binary.'
  }
];

export const TOOL_CATEGORIES: ToolCategory[] = [
  {
    title: 'Core & System',
    tools: [
      { name: 'connect(host,...)', description: 'Open SSH connection. Alias auto-generated as user@host if not provided.' },
      { name: 'disconnect(alias)', description: 'Close one or all SSH connections.' },
      { name: 'identity()', description: 'Get server\'s public key for authorized_keys.' },
      { name: 'info()', description: 'Get remote OS/kernel/shell information.' },
      { name: 'run(command)', description: 'Execute any shell command (with timeout support).' },
    ]
  },
  {
    title: 'File Operations',
    tools: [
      { name: 'read(path)', description: 'Read remote file content via native SFTP.' },
      { name: 'write(path, content)', description: 'Create/overwrite remote file.' },
      { name: 'edit(path, old, new)', description: 'Safe text replacement in a file.' },
      { name: 'list_dir(path)', description: 'List directory contents.' },
      { name: 'sync(src, dst, ...)', description: 'Stream file directly between two remote nodes.' },
    ]
  },
  {
    title: 'DevOps & Monitoring',
    tools: [
      { name: 'search_files(pattern)', description: 'Find files using POSIX find.' },
      { name: 'search_text(pattern)', description: 'Search in files using grep.' },
      { name: 'package_manage(pkg)', description: 'Install/remove packages (apt, apk, dnf, yum).' },
      { name: 'diagnose_system()', description: 'One-click SRE health check (Load, OOM, Disk).' },
      { name: 'journal_read()', description: 'Read system logs (systemd/syslog).' },
      { name: 'docker_ps()', description: 'List Docker containers.' },
    ]
  },
  {
    title: 'Database & VoIP',
    tools: [
      { name: 'db_query(...)', description: 'Execute SQL/CQL/Mongo in container.' },
      { name: 'voip_sip_capture(...)', description: 'Capture SIP/RTP packets (PCAP analysis).' },
      { name: 'list_db_containers()', description: 'Find database containers.' },
    ]
  }
];

export const CONFIG_ITEMS: ConfigItem[] = [
  { variable: 'Transport Mode', type: 'String', default: 'http', description: 'Env: SSH_MCP_MODE | Flag: --mode. Set to "stdio" or "http".' },
  { variable: 'Port', type: 'Integer', default: '8000', description: 'Env: PORT | Flag: --port. HTTP server listening port.' },
  { variable: 'Debug', type: 'Boolean', default: 'false', description: 'Env: SSH_MCP_DEBUG | Flag: --debug. Enable verbose logging.' },
  { variable: 'Global State', type: 'Boolean', default: 'false', description: 'Env: SSH_MCP_GLOBAL | Flag: --global. Share connection pool.' },
];