<script setup lang="ts">
import { CodeBlockLowlight } from '@tiptap/extension-code-block-lowlight'
import { Placeholder } from '@tiptap/extension-placeholder'
import { StarterKit } from '@tiptap/starter-kit'
import { EditorContent, useEditor } from '@tiptap/vue-3'
import {
  BoldIcon,
  CheckIcon,
  Code2Icon,
  Heading2Icon,
  Heading3Icon,
  ItalicIcon,
  LinkIcon,
  ListIcon,
  ListOrderedIcon,
  MinusIcon,
  PilcrowIcon,
  QuoteIcon,
  Redo2Icon,
  RemoveFormattingIcon,
  StrikethroughIcon,
  Undo2Icon,
  UnlinkIcon,
} from '@lucide/vue'
import { ref, watch } from 'vue'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Separator } from '@/components/ui/separator'
import { Toggle } from '@/components/ui/toggle'
import { cn } from '@/lib/utils'
import { sanitizeRichText } from '@/utils/richText'
import { lowlight } from '@/utils/syntaxHighlight'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  compact?: boolean
  invalid?: boolean
}>(), {
  placeholder: 'Start writing…',
  compact: false,
  invalid: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const linkOpen = ref(false)
const linkUrl = ref('')

const editor = useEditor({
  content: sanitizeRichText(props.modelValue),
  extensions: [
    StarterKit.configure({
      codeBlock: false,
      heading: { levels: [2, 3] },
      link: {
        autolink: true,
        defaultProtocol: 'https',
        openOnClick: false,
      },
    }),
    CodeBlockLowlight.configure({
      lowlight,
      enableTabIndentation: true,
      tabSize: 2,
    }),
    Placeholder.configure({ placeholder: props.placeholder }),
  ],
  editorProps: {
    attributes: {
      class: cn('rich-text focus:outline-none', props.compact ? 'min-h-28 px-4 py-3' : 'min-h-80 px-5 py-4'),
    },
  },
  onUpdate: ({ editor: currentEditor }) => {
    emit('update:modelValue', currentEditor.isEmpty ? '' : currentEditor.getHTML())
  },
})

watch(() => props.modelValue, (value) => {
  if (!editor.value) return
  const nextContent = sanitizeRichText(value)
  if (editor.value.getHTML() === nextContent) return
  editor.value.commands.setContent(nextContent, { emitUpdate: false })
})

function openLinkPopover() {
  linkUrl.value = editor.value?.getAttributes('link').href ?? ''
  linkOpen.value = true
}

function applyLink() {
  if (!editor.value) return
  const value = linkUrl.value.trim()
  if (!value) {
    editor.value.chain().focus().extendMarkRange('link').unsetLink().run()
    linkOpen.value = false
    return
  }

  const href = /^(?:https?:\/\/|mailto:)/i.test(value) ? value : `https://${value}`
  editor.value.chain().focus().extendMarkRange('link').setLink({ href }).run()
  linkOpen.value = false
}

function removeLink() {
  editor.value?.chain().focus().extendMarkRange('link').unsetLink().run()
  linkOpen.value = false
}
</script>

<template>
  <div :class="cn('rich-text-editor overflow-hidden rounded-xl border bg-background', invalid && 'border-destructive ring-3 ring-destructive/20')">
    <div v-if="editor" class="flex flex-wrap items-center gap-1 border-b bg-muted/40 p-2" role="toolbar" aria-label="Text formatting">
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button variant="ghost" size="sm" type="button">
            <PilcrowIcon data-icon="inline-start" />
            Style
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuGroup>
            <DropdownMenuItem @select="editor.chain().focus().setParagraph().run()">
              <PilcrowIcon /><span>Paragraph</span><CheckIcon v-if="editor.isActive('paragraph')" class="ml-auto" />
            </DropdownMenuItem>
            <DropdownMenuItem @select="editor.chain().focus().toggleHeading({ level: 2 }).run()">
              <Heading2Icon /><span>Heading 2</span><CheckIcon v-if="editor.isActive('heading', { level: 2 })" class="ml-auto" />
            </DropdownMenuItem>
            <DropdownMenuItem @select="editor.chain().focus().toggleHeading({ level: 3 }).run()">
              <Heading3Icon /><span>Heading 3</span><CheckIcon v-if="editor.isActive('heading', { level: 3 })" class="ml-auto" />
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>

      <Separator orientation="vertical" class="mx-1 h-7" />

      <Toggle :model-value="editor.isActive('bold')" size="sm" aria-label="Bold" title="Bold" @update:model-value="editor.chain().focus().toggleBold().run()"><BoldIcon /></Toggle>
      <Toggle :model-value="editor.isActive('italic')" size="sm" aria-label="Italic" title="Italic" @update:model-value="editor.chain().focus().toggleItalic().run()"><ItalicIcon /></Toggle>
      <Toggle :model-value="editor.isActive('strike')" size="sm" aria-label="Strikethrough" title="Strikethrough" @update:model-value="editor.chain().focus().toggleStrike().run()"><StrikethroughIcon /></Toggle>
      <Toggle :model-value="editor.isActive('code')" size="sm" aria-label="Inline code" title="Inline code" @update:model-value="editor.chain().focus().toggleCode().run()"><Code2Icon /></Toggle>

      <Separator orientation="vertical" class="mx-1 h-7" />

      <Toggle :model-value="editor.isActive('bulletList')" size="sm" aria-label="Bullet list" title="Bullet list" @update:model-value="editor.chain().focus().toggleBulletList().run()"><ListIcon /></Toggle>
      <Toggle :model-value="editor.isActive('orderedList')" size="sm" aria-label="Numbered list" title="Numbered list" @update:model-value="editor.chain().focus().toggleOrderedList().run()"><ListOrderedIcon /></Toggle>
      <Toggle :model-value="editor.isActive('blockquote')" size="sm" aria-label="Quote" title="Quote" @update:model-value="editor.chain().focus().toggleBlockquote().run()"><QuoteIcon /></Toggle>
      <Toggle :model-value="editor.isActive('codeBlock')" size="sm" aria-label="Code block" title="Code block" @update:model-value="editor.chain().focus().toggleCodeBlock().run()"><Code2Icon /></Toggle>

      <Popover v-model:open="linkOpen">
        <PopoverTrigger as-child>
          <Toggle :model-value="editor.isActive('link')" size="sm" aria-label="Link" title="Link" @click="openLinkPopover"><LinkIcon /></Toggle>
        </PopoverTrigger>
        <PopoverContent align="start">
          <PopoverHeader>
            <PopoverTitle>Add a link</PopoverTitle>
            <PopoverDescription>Paste a web or email address.</PopoverDescription>
          </PopoverHeader>
          <FieldGroup>
            <Field>
              <FieldLabel for="rich-text-link">URL</FieldLabel>
              <Input id="rich-text-link" v-model="linkUrl" type="url" placeholder="https://example.com" @keydown.enter.prevent="applyLink" />
            </Field>
          </FieldGroup>
          <div class="flex justify-end gap-2">
            <Button v-if="editor.isActive('link')" variant="outline" size="sm" type="button" @click="removeLink"><UnlinkIcon data-icon="inline-start" />Remove</Button>
            <Button size="sm" type="button" @click="applyLink"><LinkIcon data-icon="inline-start" />Apply</Button>
          </div>
        </PopoverContent>
      </Popover>

      <Button variant="ghost" size="icon-sm" type="button" aria-label="Horizontal rule" title="Horizontal rule" @click="editor.chain().focus().setHorizontalRule().run()"><MinusIcon /></Button>
      <Button variant="ghost" size="icon-sm" type="button" aria-label="Clear formatting" title="Clear formatting" @click="editor.chain().focus().unsetAllMarks().clearNodes().run()"><RemoveFormattingIcon /></Button>

      <Separator orientation="vertical" class="mx-1 h-7" />

      <Button variant="ghost" size="icon-sm" type="button" aria-label="Undo" title="Undo" :disabled="!editor.can().chain().focus().undo().run()" @click="editor.chain().focus().undo().run()"><Undo2Icon /></Button>
      <Button variant="ghost" size="icon-sm" type="button" aria-label="Redo" title="Redo" :disabled="!editor.can().chain().focus().redo().run()" @click="editor.chain().focus().redo().run()"><Redo2Icon /></Button>
    </div>
    <EditorContent :editor="editor" :aria-invalid="invalid || undefined" />
  </div>
</template>
