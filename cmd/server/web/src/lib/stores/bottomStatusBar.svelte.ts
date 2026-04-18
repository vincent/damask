
let slot1 = $state<string | null>(null)
let slot2 = $state<string | null>(null)
let slot3 = $state<string | null>(null)

let showZoom = $state(false)
let maxZoom = $state(20)
let zoom = $state(10)

let lang = $state('en')

export const statusBarStore = {
    get slots(){ return [slot1, slot2, slot3].filter(Boolean) },
    get slot1(){ return slot1 },
    set slot1(z){ slot1 = z },
    get slot2(){ return slot2 },
    set slot2(z){ slot1 = z },
    get slot3(){ return slot3 },
    set slot3(z){ slot1 = z },

    get showZoom(){ return showZoom },
    set showZoom(z){ showZoom = z },
    get maxZoom() { return maxZoom; },
    get sliderMax() { return maxZoom - 1; },
    get zoom(){ return zoom },
    set zoom(z){ zoom = z },

    get lang(){ return lang },
    set lang(z){ lang = z },
}
