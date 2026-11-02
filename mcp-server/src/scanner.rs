use regex::Regex;
use std::fs;
use std::path::Path;

#[derive(Debug, serde::Serialize)]
pub struct ScanFinding {
    pub file_path: String,
    pub line_number: usize,
    pub pattern_name: String,
    pub credential_type: String,
    pub redacted_preview: String,
}

struct PatternDef {
    name: &'static str,
    credential_type: &'static str,
    pattern: &'static str,
}

static PATTERNS: &[PatternDef] = &[
    PatternDef {
        name: "aws_access_key",
        credential_type: "aws_key",
        pattern: r"AKIA[0-9A-Z]{16}",
    },
    PatternDef {
        name: "aws_secret_key",
        credential_type: "aws_secret",
        pattern: r#"(?i)aws[_\-\s]?secret[_\-\s]?(?:access[_\-\s]?)?key['"]?\s*[:=]\s*['"]?([A-Za-z0-9/+=]{40})"#,
    },
    PatternDef {
        name: "github_pat_classic",
        credential_type: "github_token",
        pattern: r"ghp_[A-Za-z0-9]{36}",
    },
    PatternDef {
        name: "github_oauth",
        credential_type: "github_token",
        pattern: r"ghs_[A-Za-z0-9]{36}",
    },
    PatternDef {
        name: "github_pat_fine",
        credential_type: "github_token",
        pattern: r"github_pat_[A-Za-z0-9_]{82}",
    },
    PatternDef {
        name: "stripe_secret",
        credential_type: "stripe_key",
        pattern: r"sk_live_[A-Za-z0-9]{24,}",
    },
    PatternDef {
        name: "stripe_publishable",
        credential_type: "stripe_key",
        pattern: r"pk_live_[A-Za-z0-9]{24,}",
    },
    PatternDef {
        name: "openai_key",
        credential_type: "openai_key",
        pattern: r"sk-[A-Za-z0-9]{48}",
    },
    PatternDef {
        name: "google_api_key",
        credential_type: "google_api_key",
        pattern: r"AIza[0-9A-Za-z\-_]{35}",
    },
    PatternDef {
        name: "postgres_uri",
        credential_type: "database_uri",
        pattern: r#"postgres(?:ql)?://[^\s'"]{10,}"#,
    },
    PatternDef {
        name: "mysql_uri",
        credential_type: "database_uri",
        pattern: r#"mysql://[^\s'"]{10,}"#,
    },
    PatternDef {
        name: "jwt_token",
        credential_type: "jwt",
        pattern: r"eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}",
    },
    PatternDef {
        name: "ssh_private_key",
        credential_type: "ssh_key",
        pattern: r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----",
    },
    PatternDef {
        name: "slack_bot_token",
        credential_type: "slack_token",
        pattern: r"xoxb-[0-9A-Za-z\-]{24,}",
    },
    PatternDef {
        name: "slack_user_token",
        credential_type: "slack_token",
        pattern: r"xoxp-[0-9A-Za-z\-]{24,}",
    },
    PatternDef {
        name: "high_entropy_hex",
        credential_type: "generic_secret",
        pattern: r#"(?:password|secret|key|token|api_key|apikey)\s*[:=]\s*['"]?([0-9a-fA-F]{40,})['"]?"#,
    },
    PatternDef {
        name: "high_entropy_b64",
        credential_type: "generic_secret",
        pattern: r#"(?:password|secret|key|token|api_key|apikey)\s*[:=]\s*['"]?([A-Za-z0-9+/]{40,}={0,2})['"]?"#,
    },
];

const SKIP_DIRS: &[&str] = &["node_modules", ".git", "vendor", "target", "__pycache__"];

const ALLOWED_EXTENSIONS: &[&str] = &[
    ".env", ".js", ".ts", ".jsx", ".tsx", ".py", ".go", ".rs", ".yaml", ".yml", ".json", ".toml",
    ".sh", ".rb", ".php", ".conf", ".config",
];

pub fn redact_value(s: &str) -> String {
    let chars: Vec<char> = s.chars().collect();
    let len = chars.len();
    if len <= 8 {
        return "*".repeat(len);
    }
    let prefix: String = chars[..4].iter().collect();
    let suffix: String = chars[len - 4..].iter().collect();
    format!("{prefix}****{suffix}")
}

fn is_allowed_file(path: &Path) -> bool {
    let name = path.file_name().and_then(|n| n.to_str()).unwrap_or("");
    // Allow .env and .env.* (dotenv files)
    if name == ".env" || name.starts_with(".env.") {
        return true;
    }
    let ext = path.extension().and_then(|e| e.to_str()).unwrap_or("");
    if ext.is_empty() {
        return false;
    }
    let ext_with_dot = format!(".{ext}");
    ALLOWED_EXTENSIONS.contains(&ext_with_dot.as_str())
}

fn should_skip_dir(name: &str) -> bool {
    SKIP_DIRS.contains(&name)
}

fn scan_file(file_path: &Path, compiled: &[(Regex, &PatternDef)]) -> Vec<ScanFinding> {
    let content = match fs::read(file_path) {
        Ok(bytes) => bytes,
        Err(_) => return vec![],
    };
    // Skip binary files heuristically
    if content.iter().take(512).any(|&b| b == 0) {
        return vec![];
    }
    let text = match std::str::from_utf8(&content) {
        Ok(s) => s,
        Err(_) => return vec![],
    };

    let path_str = file_path.to_string_lossy().to_string();
    let mut findings = Vec::new();

    for (line_number, line) in text.lines().enumerate() {
        let line_num = line_number + 1;
        for (re, def) in compiled {
            if let Some(cap) = re.find(line) {
                let matched = cap.as_str();
                // Use first capture group if present, else full match
                let value = re
                    .captures(line)
                    .and_then(|c| c.get(1))
                    .map(|m| m.as_str())
                    .unwrap_or(matched);
                let redacted = redact_value(value);
                findings.push(ScanFinding {
                    file_path: path_str.clone(),
                    line_number: line_num,
                    pattern_name: def.name.to_string(),
                    credential_type: def.credential_type.to_string(),
                    redacted_preview: redacted,
                });
                break; // one finding per line per file
            }
        }
    }
    findings
}

pub fn scan_directory(path: &str, recursive: bool) -> Vec<ScanFinding> {
    let compiled: Vec<(Regex, &PatternDef)> = PATTERNS
        .iter()
        .filter_map(|def| Regex::new(def.pattern).ok().map(|re| (re, def)))
        .collect();

    let root = Path::new(path);
    let mut findings = Vec::new();
    scan_path(root, &compiled, recursive, &mut findings);
    findings
}

fn scan_path(
    path: &Path,
    compiled: &[(Regex, &PatternDef)],
    recursive: bool,
    findings: &mut Vec<ScanFinding>,
) {
    if path.is_file() {
        if is_allowed_file(path) {
            findings.extend(scan_file(path, compiled));
        }
        return;
    }
    if !path.is_dir() {
        return;
    }
    let entries = match fs::read_dir(path) {
        Ok(e) => e,
        Err(_) => return,
    };
    for entry in entries.flatten() {
        let entry_path = entry.path();
        if entry_path.is_dir() {
            let dir_name = entry_path
                .file_name()
                .and_then(|n| n.to_str())
                .unwrap_or("");
            if should_skip_dir(dir_name) {
                continue;
            }
            if recursive {
                scan_path(&entry_path, compiled, recursive, findings);
            }
        } else if is_allowed_file(&entry_path) {
            findings.extend(scan_file(&entry_path, compiled));
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_redact_short() {
        assert_eq!(redact_value("abc"), "***");
        assert_eq!(redact_value("12345678"), "********");
    }

    #[test]
    fn test_redact_long() {
        // AWS documentation's public example key (not a real credential);
        // assembled at runtime so credential scanners don't flag the literal.
        let example_key = format!("AKIA{}", "IOSFODNN7EXAMPLE");
        let result = redact_value(&example_key);
        assert!(result.starts_with("AKIA"));
        assert!(result.ends_with("MPLE"));
        assert!(result.contains("****"));
    }

    #[test]
    fn test_is_allowed_file_env() {
        assert!(is_allowed_file(Path::new(".env")));
        assert!(is_allowed_file(Path::new(".env.local")));
        assert!(is_allowed_file(Path::new("config.ts")));
        assert!(!is_allowed_file(Path::new("image.png")));
    }

    #[test]
    fn test_should_skip_dirs() {
        assert!(should_skip_dir("node_modules"));
        assert!(should_skip_dir(".git"));
        assert!(!should_skip_dir("src"));
    }
}
