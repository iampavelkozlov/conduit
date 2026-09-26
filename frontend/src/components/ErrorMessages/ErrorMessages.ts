import { defineComponent } from 'vue'
import { CircleAlertIcon } from '@lucide/vue'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

export default defineComponent({
  name: 'ErrorMessages',
  components: { Alert, AlertDescription, AlertTitle, CircleAlertIcon },
  props: {
    messages: { type: Array<string>, required: true },
  },
})
