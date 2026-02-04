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
    commands: `docker run -d \\
  --name ssh-mcp \\
  -p 8000:8000 \\
  -v ssh-mcp-data:/data \\
  firstfinger/ssh-mcp:latest`,
    note: 'Distroless production image (11MB)'
  },
  {
    id: 'compose',
    label: 'docker-compose.yml',
    commands: `services:
  ssh-mcp:
    image: firstfinger/ssh-mcp:latest
    container_name: ssh-mcp
    ports:
      - "8000:8000"
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

# Run
./ssh-mcp`,
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
  { variable: 'PORT', type: 'Integer', default: '8000', description: 'The port the HTTP server listens on.' },
  { variable: 'SSH_MCP_SESSION_HEADER', type: 'String', default: 'X-Session-Key', description: 'Header used for smart session caching.' },
  { variable: 'SSH_MCP_SESSION_TIMEOUT', type: 'Integer', default: '300', description: 'Idle timeout for cached sessions in seconds.' },
  { variable: 'SSH_MCP_GLOBAL_STATE', type: 'Boolean', default: 'false', description: 'If true, a single SSH manager is shared by all clients.' },
  { variable: 'SSH_MCP_COMMAND_TIMEOUT', type: 'Float', default: '120.0', description: 'Maximum time (seconds) allowed for an SSH command.' },
  { variable: 'SSH_MCP_MAX_OUTPUT', type: 'Integer', default: '51200', description: 'Maximum byte size of command output returned.' },
];