export interface Tool {
  name: string;
  description: string;
}

export interface ToolCategory {
  title: string;
  tools: Tool[];
}

export interface ConfigItem {
  variable: string;
  type: string;
  default: string;
  description: string;
}

export interface InstallMethod {
  id: string;
  label: string;
  commands: string;
  note?: string;
}

export interface NavItem {
  label: string;
  href: string;
}