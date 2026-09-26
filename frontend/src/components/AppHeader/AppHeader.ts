import { storeToRefs } from 'pinia'
import { defineComponent } from 'vue'
import { useRouter } from 'vue-router'
import { BookOpenIcon, HomeIcon, LogInIcon, LogOutIcon, MenuIcon, PenLineIcon, SettingsIcon, UserPlusIcon } from '@lucide/vue'

import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { useAuthStore } from '../../stores/auth'
import { avatarUrl, initials } from '../../utils/format'

export default defineComponent({
  name: 'AppHeader',
  components: {
    Avatar,
    AvatarFallback,
    AvatarImage,
    BookOpenIcon,
    Button,
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuGroup,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
    HomeIcon,
    LogInIcon,
    LogOutIcon,
    MenuIcon,
    PenLineIcon,
    SettingsIcon,
    Sheet,
    SheetClose,
    SheetContent,
    SheetDescription,
    SheetHeader,
    SheetTitle,
    SheetTrigger,
    UserPlusIcon,
  },
  setup() {
    const auth = useAuthStore()
    const router = useRouter()
    const { user } = storeToRefs(auth)

    function logout() {
      auth.logout()
      void router.push({ name: 'home' })
    }

    return { user, logout, avatarUrl, initials }
  },
})
