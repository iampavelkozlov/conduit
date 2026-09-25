import { defineComponent } from 'vue'

export default defineComponent({
  name: 'ErrorMessages',
  props: {
    messages: { type: Array<string>, required: true },
  },
})
