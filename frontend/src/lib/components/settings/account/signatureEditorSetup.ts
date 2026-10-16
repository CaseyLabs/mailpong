import { Editor, Extension } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import Image from '@tiptap/extension-image'
import { FontSize } from '@tiptap/extension-text-style'
import TextAlign from '@tiptap/extension-text-align'
import { TableRow } from '@tiptap/extension-table'
import {
  ExtendedTextStyle,
  ExtendedColor,
  ExtendedTable,
  ExtendedTableCell,
  ExtendedTableHeader,
} from '$lib/components/composer/tiptapExtensions'

// Custom extension to make Enter insert <br> instead of new paragraph
// Optimized with direct ProseMirror transaction for better performance
const LineBreakOnEnter = Extension.create({
  name: 'lineBreakOnEnter',
  priority: 1000,
  addKeyboardShortcuts() {
    return {
      Enter: () => {
        const { view } = this.editor
        const { state } = view
        const { schema, selection, storedMarks } = state

        const hardBreakType = schema.nodes.hardBreak
        if (!hardBreakType) return false

        const fromPos = selection.$from
        if (fromPos.parent.type.spec.isolating) return false

        const marks = storedMarks ||
          (selection.$to.parentOffset && selection.$from.marks()) ||
          []

        let tr = state.tr.replaceSelectionWith(hardBreakType.create(), false)

        if (marks.length > 0) {
          tr = tr.ensureMarks(marks)
        }

        tr.scrollIntoView()
        view.dispatch(tr)
        return true
      },
    }
  },
})


export interface SignatureEditorOptions {
  element: HTMLElement | undefined
  placeholder: string
  content: string
  onImageFile: (file: File) => void
  onUpdate: () => void
  onTransaction: () => void
}

export function createSignatureEditor(opts: SignatureEditorOptions): Editor {
  return new Editor({
    element: opts.element,
    extensions: [
      StarterKit.configure({
        // Disable heading for signatures - keep it simple
        heading: false,
        link: {
          openOnClick: false,
          HTMLAttributes: {
            class: 'text-primary underline',
          },
        },
        trailingNode: false,
      }),
      LineBreakOnEnter,  // Make Enter insert <br> instead of new paragraph
      ExtendedTextStyle,  // Required for Color extension (extended for better paste support)
      ExtendedColor,      // Text color support (extended for font tags and inline styles)
      FontSize,           // Font size support
      TextAlign.configure({
        types: ['paragraph'],  // Apply to paragraphs
      }),
      // Table support
      ExtendedTable.configure({
        resizable: false,
        HTMLAttributes: {
          class: 'border-collapse',
        },
      }),
      TableRow,
      ExtendedTableCell,
      ExtendedTableHeader,
      Image.configure({
        inline: true,
        allowBase64: true,
        HTMLAttributes: {
          class: 'max-w-full h-auto',
        },
      }),
      Placeholder.configure({
        placeholder: opts.placeholder,
      }),
    ],
    content: opts.content,
    editorProps: {
      attributes: {
        class: 'signature-editor focus:outline-hidden min-h-[100px] p-3',
      },
      // Handle paste events for images
      handlePaste: (view, event) => {
        const items = event.clipboardData?.items
        if (!items) return false

        for (const item of items) {
          if (item.type.startsWith('image/')) {
            event.preventDefault()
            const file = item.getAsFile()
            if (file) {
              opts.onImageFile(file)
            }
            return true
          }
        }
        // Allow HTML paste (for importing signatures)
        return false
      },
      // Handle drop events for images
      handleDrop: (view, event, slice, moved) => {
        if (moved) return false

        const files = event.dataTransfer?.files
        if (!files?.length) return false

        for (const file of files) {
          if (file.type.startsWith('image/')) {
            event.preventDefault()
            opts.onImageFile(file)
            return true
          }
        }
        return false
      },
    },
    onUpdate: opts.onUpdate,
    onTransaction: opts.onTransaction,
  })
}
