import { defineComponent } from 'vue'

import AppFooter from '../../components/AppFooter/AppFooter.vue'
import AppHeader from '../../components/AppHeader/AppHeader.vue'

export default defineComponent({
  name: 'DefaultLayout',
  components: { AppFooter, AppHeader },
})
