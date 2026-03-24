use serde::{Deserialize, Serialize};

/// Content age rating.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum Rating {
    #[serde(rename = "6+")]
    Age6Plus,
    #[serde(rename = "12+")]
    Age12Plus,
    #[serde(rename = "16+")]
    Age16Plus,
    #[serde(rename = "18+")]
    Age18Plus,
    #[serde(rename = "BLOCKED")]
    Blocked,
}

/// Classification result.
#[derive(Debug, Serialize, Deserialize)]
pub struct ContentRating {
    pub rating: Rating,
    pub safe: bool,
    pub score: u8,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub flags: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<String>,
}

struct Category {
    name: &'static str,
    keywords: &'static [&'static str],
    severity: u8, // 0=safe, 1=mild, 2=mature, 3=adult, 4=blocked
}

static CATEGORIES: &[Category] = &[
    Category {
        name: "violence",
        keywords: &["kill", "murder", "weapon", "gun", "knife", "attack",
            "fight", "bomb", "explode", "assault", "torture", "blood"],
        severity: 3,
    },
    Category {
        name: "mild_conflict",
        keywords: &["battle", "war", "conflict", "combat", "strategy",
            "military", "defense", "soldier"],
        severity: 1,
    },
    Category {
        name: "drugs",
        keywords: &["drug", "narcotic", "cocaine", "heroin", "meth",
            "marijuana", "overdose", "substance abuse"],
        severity: 3,
    },
    Category {
        name: "adult_content",
        keywords: &["explicit", "nsfw", "porn", "sexual", "erotic", "nude", "fetish"],
        severity: 4,
    },
    Category {
        name: "security_tools",
        keywords: &["exploit", "vulnerability", "hack", "injection",
            "brute force", "phishing", "malware", "ransomware", "backdoor"],
        severity: 2,
    },
    Category {
        name: "security_educational",
        keywords: &["security", "penetration test", "ctf", "defense",
            "firewall", "encryption", "authentication"],
        severity: 1,
    },
    Category {
        name: "illegal",
        keywords: &["illegal", "fraud", "counterfeit", "launder",
            "trafficking", "steal", "theft"],
        severity: 4,
    },
    Category {
        name: "gambling",
        keywords: &["gambling", "casino", "bet", "poker", "slot machine", "lottery"],
        severity: 2,
    },
];

static EDUCATIONAL_MARKERS: &[&str] = &[
    "explain", "how does", "what is", "learn", "teach", "understand",
    "example", "homework", "school", "university", "research", "study",
    "tutorial", "course", "lesson",
];

/// Classify content and return age rating.
pub fn classify(text: &str) -> ContentRating {
    let lower = text.to_lowercase();
    let mut max_severity: u8 = 0;
    let mut flags = Vec::new();

    let is_educational = EDUCATIONAL_MARKERS.iter().any(|m| lower.contains(m));

    for cat in CATEGORIES {
        let matched = cat.keywords.iter().any(|kw| lower.contains(kw));
        if matched {
            flags.push(cat.name.to_string());
            let mut severity = cat.severity;
            if is_educational && severity > 0 {
                severity -= 1;
            }
            if severity > max_severity {
                max_severity = severity;
            }
        }
    }

    let (rating, safe, details) = match max_severity {
        4.. => (Rating::Blocked, false, Some("content policy violation detected".to_string())),
        3 => (Rating::Age18Plus, false, Some("adult content detected".to_string())),
        2 => (Rating::Age16Plus, true, Some("mature themes detected".to_string())),
        1 => (Rating::Age12Plus, true, Some("mild themes detected".to_string())),
        _ => (Rating::Age6Plus, true, None),
    };

    ContentRating {
        rating,
        safe,
        score: max_severity,
        flags,
        details,
    }
}

/// Check if content is safe for given max rating.
pub fn is_safe(text: &str, max_rating: &Rating) -> bool {
    let result = classify(text);
    rating_level(&result.rating) <= rating_level(max_rating)
}

fn rating_level(r: &Rating) -> u8 {
    match r {
        Rating::Age6Plus => 0,
        Rating::Age12Plus => 1,
        Rating::Age16Plus => 2,
        Rating::Age18Plus => 3,
        Rating::Blocked => 4,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_safe_content() {
        let r = classify("Help me with my math homework");
        assert_eq!(r.rating, Rating::Age6Plus);
        assert!(r.safe);
    }

    #[test]
    fn test_security_educational() {
        let r = classify("Explain how SQL injection attacks work for my security course");
        assert_eq!(r.rating, Rating::Age16Plus);
        assert!(r.safe);
    }

    #[test]
    fn test_mild_conflict() {
        let r = classify("Write a story about a battle in ancient Rome");
        assert_eq!(r.rating, Rating::Age12Plus);
        assert!(r.safe);
    }
}
