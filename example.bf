
(ns myspace)

(func main
    (var i 3)
    (as i (add 5 3))
    (as i (kevin 2)))

(func kevin I : a I
    (return (add a 4)))

(class Foo
    (f alice I 24)

    (m bar F : a I c I
        (var b Foo (Foo))
        //(var i I [alice])
        (as [alice] 9)
        (var ack [alice])
        (return 3.0)))


(class Monkey : Bar Eater

    (m david
        (var i 3)
        (as i 5)
        (as [zelda] 6.0)
        (lisa me)
    )
)

(interface Eater

    (m david)

)


(class Bar : Foo
    (f zelda F 35.0)

    (m lisa
        (var i 3)
        (as i 5)
        (var test Eater)
    )

    (constructor 
        (var i 3)
        (as i 5)
    )

    (constructor : a Str
        (var i 3)
        (as i 5)
    )
)


// (class HeadingTargetRandomizerSystem : ComponentSystem ISomeInterface
//     (@ Inject)
//     (f -priv group Group)

//     // property with getter and setter (getter starts first; setter body separated by -set)
//     (p hours FF
//         (return (div second 3600))
//         -set           
//         (if (or (lt value 0) (gt value 24))
//             (throw (ArgumentOutOfRangeException $`{nameof(value)} must be between 0 and 24.`))
//         )
//         (as seconds (mul value 3600))
//     )

//     (m -prot -over onUpdate
//         (forinc i 0 [group length]
//             (var entity [group entities i])          // i is ambiguous because some types use both . and []
//             (setComponent postUpdateCommands entity (HeadingTarget onUnitSphere/random))
//         )
//     )
// )

// (struct -priv Group
//     (@ WriteOnly) 
//     (f randomizeHeadings ComponentDataArray<RandomizeHeadingTarget>)    
//     (@ ReadOnly) 
//     (f randomizeHeadings ComponentDataArray<RandomizeHeadingTarget>)
//     (f entities EntityArray)
//     (f length I)
// )



// E<Str>         // like Option<Str>: maybe a string or maybe an error