/// Generate a shields.io-style flat badge SVG for an archlint score.
///
/// Colors:
/// - green  #4c1   (score > 80)
/// - yellow #dfb317 (score 50-80)
/// - red    #e05d44 (score < 50)
pub fn generate_badge(score: u32) -> String {
    let score = score.min(100);
    let color = if score > 80 {
        "#4c1"
    } else if score >= 50 {
        "#dfb317"
    } else {
        "#e05d44"
    };

    let label = "archlint";
    let value = format!("{}/100", score);

    // Approximate text widths (~6.5px per char at font-size 11, in tenths of px)
    let label_text_width = label.len() as u32 * 65;
    let value_text_width = value.len() as u32 * 65;

    // Padding: 5px each side
    let label_width = label_text_width / 10 + 10;
    let value_width = value_text_width / 10 + 10;
    let total_width = label_width + value_width;

    // Center x positions (in tenths of px, because text is scaled 0.1)
    let label_x = label_width * 10 / 2;
    let value_x = label_width * 10 + value_width * 10 / 2;

    // Build SVG by concatenation to avoid Rust 2021 prefix issues with # in format strings
    let mut svg = String::new();
    svg.push_str(&format!(
        "<svg xmlns=\"http://www.w3.org/2000/svg\" xmlns:xlink=\"http://www.w3.org/1999/xlink\" width=\"{}\" height=\"20\">\n",
        total_width
    ));
    svg.push_str("  <linearGradient id=\"s\" x2=\"0\" y2=\"100%\">\n");
    svg.push_str("    <stop offset=\"0\" stop-color=\"#bbb\" stop-opacity=\".1\"/>\n");
    svg.push_str("    <stop offset=\"1\" stop-opacity=\".1\"/>\n");
    svg.push_str("  </linearGradient>\n");
    svg.push_str("  <clipPath id=\"r\">\n");
    svg.push_str(&format!(
        "    <rect width=\"{}\" height=\"20\" rx=\"3\" fill=\"#fff\"/>\n",
        total_width
    ));
    svg.push_str("  </clipPath>\n");
    svg.push_str("  <g clip-path=\"url(#r)\">\n");
    svg.push_str(&format!(
        "    <rect width=\"{}\" height=\"20\" fill=\"#555\"/>\n",
        label_width
    ));
    svg.push_str(&format!(
        "    <rect x=\"{}\" width=\"{}\" height=\"20\" fill=\"{}\"/>\n",
        label_width, value_width, color
    ));
    svg.push_str(&format!(
        "    <rect width=\"{}\" height=\"20\" fill=\"url(#s)\"/>\n",
        total_width
    ));
    svg.push_str("  </g>\n");
    svg.push_str("  <g fill=\"#fff\" text-anchor=\"middle\" font-family=\"DejaVu Sans,Verdana,Geneva,sans-serif\" font-size=\"110\">\n");
    svg.push_str(&format!(
        "    <text x=\"{}\" y=\"150\" fill=\"#010101\" fill-opacity=\".3\" transform=\"scale(.1)\" textLength=\"{}\" lengthAdjust=\"spacing\">{}</text>\n",
        label_x, label_text_width, label
    ));
    svg.push_str(&format!(
        "    <text x=\"{}\" y=\"140\" transform=\"scale(.1)\" textLength=\"{}\" lengthAdjust=\"spacing\">{}</text>\n",
        label_x, label_text_width, label
    ));
    svg.push_str(&format!(
        "    <text x=\"{}\" y=\"150\" fill=\"#010101\" fill-opacity=\".3\" transform=\"scale(.1)\" textLength=\"{}\" lengthAdjust=\"spacing\">{}</text>\n",
        value_x, value_text_width, value
    ));
    svg.push_str(&format!(
        "    <text x=\"{}\" y=\"140\" transform=\"scale(.1)\" textLength=\"{}\" lengthAdjust=\"spacing\">{}</text>\n",
        value_x, value_text_width, value
    ));
    svg.push_str("  </g>\n");
    svg.push_str("</svg>");
    svg
}

/// Calculate score from violation count: 100 - (violations * 5), clamped to 0-100.
pub fn score_from_violations(violations: usize) -> u32 {
    let penalty = violations * 5;
    if penalty >= 100 {
        0
    } else {
        (100 - penalty) as u32
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_green_badge() {
        let svg = generate_badge(90);
        assert!(svg.contains("#4c1"), "score 90 should be green");
        assert!(svg.contains("90/100"));
    }

    #[test]
    fn test_yellow_badge() {
        let svg = generate_badge(65);
        assert!(svg.contains("#dfb317"), "score 65 should be yellow");
        assert!(svg.contains("65/100"));
    }

    #[test]
    fn test_red_badge() {
        let svg = generate_badge(30);
        assert!(svg.contains("#e05d44"), "score 30 should be red");
        assert!(svg.contains("30/100"));
    }

    #[test]
    fn test_svg_contains_score() {
        for score in [0u32, 50, 80, 100] {
            let svg = generate_badge(score);
            assert!(svg.contains(&format!("{}/100", score)), "badge must contain score {}/100", score);
        }
    }

    #[test]
    fn test_score_boundary_green() {
        // score 81 -> green, score 80 -> yellow
        let svg81 = generate_badge(81);
        assert!(svg81.contains("#4c1"), "81 should be green");
        let svg80 = generate_badge(80);
        assert!(svg80.contains("#dfb317"), "80 should be yellow");
    }

    #[test]
    fn test_score_boundary_red() {
        // score 50 -> yellow, score 49 -> red
        let svg50 = generate_badge(50);
        assert!(svg50.contains("#dfb317"), "50 should be yellow");
        let svg49 = generate_badge(49);
        assert!(svg49.contains("#e05d44"), "49 should be red");
    }

    #[test]
    fn test_score_clamped() {
        // score > 100 should be clamped to 100
        let svg = generate_badge(200);
        assert!(svg.contains("100/100"), "score should be clamped to 100");
    }

    #[test]
    fn test_score_from_violations() {
        assert_eq!(score_from_violations(0), 100);
        assert_eq!(score_from_violations(4), 80);
        assert_eq!(score_from_violations(10), 50);
        assert_eq!(score_from_violations(20), 0);
        assert_eq!(score_from_violations(100), 0); // clamped
    }

    #[test]
    fn test_svg_is_valid_xml_structure() {
        let svg = generate_badge(75);
        assert!(svg.starts_with("<svg"), "should start with <svg");
        assert!(svg.ends_with("</svg>"), "should end with </svg>");
        assert!(svg.contains("archlint"), "should contain label");
    }
}
