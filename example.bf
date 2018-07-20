(class HeadingTargetRandomizerSystem : ComponentSystem ISomeInterface
    (struct -private Group
        (@ WriteOnly) 
        (f randomizeHeadings ComponentDataArray<RandomizeHeadingTarget>)    
        (@ ReadOnly) 
        (f randomizeHeadings ComponentDataArray<RandomizeHeadingTarget>)
        (f entities EntityArray)
        (f length I)
    )

    (@ Inject)
    (f -priv group Group)

    // property with getter and setter (getter starts first; setter body separated by -set)
    (p hours FF
        (return (div second 3600))
        -set            
        (if (or (lt value 0) (gt value 24))
            (throw (ArgumentOutOfRangeException $`{nameof(value)} must be between 0 and 24.`))
        )
        (as seconds (mul value 3600))
    )

    (m -prot -over onUpdate
        (forinc i 0 [group length]
            (var entity [group entities i])          // i is ambiguous because some types use both . and []
            (setComponent postUpdateCommands entity (HeadingTarget onUnitSphere/random))
        )
    )
)