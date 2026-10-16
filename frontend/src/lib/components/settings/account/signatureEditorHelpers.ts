// Pure helpers and constants for SignatureEditor (moved verbatim).

// Font size options
export const fontSizes = ['10px', '12px', '14px', '15px', '16px', '18px', '20px', '24px', '28px', '32px']

// Preset colors for quick selection
export const presetColors = [
  '#000000', '#374151', '#6b7280', // Grays
  '#dc2626', '#ea580c', '#ca8a04', // Warm
  '#16a34a', '#0891b2', '#2563eb', // Cool
  '#7c3aed', '#c026d3', '#e11d48', // Vibrant
]

export function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(file)
  })
}

// Pretty-print HTML with indentation for readability in raw mode
export function formatHtml(html: string): string {
  const blockTags = /^(p|div|table|thead|tbody|tr|td|th|ul|ol|li|blockquote|hr|br)$/i

  // Insert newlines around block-level tags
  const result = html
    .replace(/>\s*</g, '>\n<')  // Newline between adjacent tags
    .trim()

  const lines = result.split('\n')
  const formatted: string[] = []
  let depth = 0

  for (const rawLine of lines) {
    const line = rawLine.trim()
    if (!line) continue

    // Check if line starts with a closing block tag
    const closingMatch = line.match(/^<\/(\w+)/)
    if (closingMatch && blockTags.test(closingMatch[1])) {
      depth = Math.max(0, depth - 1)
    }

    formatted.push('  '.repeat(depth) + line)

    // Check for opening block tags (not self-closing)
    const openingMatch = line.match(/^<(\w+)/)
    if (openingMatch && blockTags.test(openingMatch[1]) && !line.match(/\/>$/) && !line.match(/^<br/i)) {
      // Only increase depth if the line doesn't also close the same tag
      const tagName = openingMatch[1]
      if (!line.includes(`</${tagName}>`)) {
        depth++
      }
    }
  }

  return formatted.join('\n')
}
